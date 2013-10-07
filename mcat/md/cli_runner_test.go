package md_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry/hm9000/config"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type CLIRunner struct {
	configPath           string
	listenerCmd          *exec.Cmd
	listenerStdoutBuffer *bytes.Buffer
	verbose              bool
}

func NewCLIRunner(storeURLs []string, ccBaseURL string, natsPort int, verbose bool) *CLIRunner {
	runner := &CLIRunner{
		verbose: verbose,
	}
	runner.generateConfig(storeURLs, ccBaseURL, natsPort)
	return runner
}

func (runner *CLIRunner) generateConfig(storeURLs []string, ccBaseURL string, natsPort int) {
	tmpFile, err := ioutil.TempFile("/tmp", "hm9000_clirunner")
	defer tmpFile.Close()
	Ω(err).ShouldNot(HaveOccured())

	runner.configPath = tmpFile.Name()

	conf, err := config.DefaultConfig()
	Ω(err).ShouldNot(HaveOccured())
	conf.StoreURLs = storeURLs
	conf.CCBaseURL = ccBaseURL
	conf.NATS.Port = natsPort
	conf.SenderMessageLimit = 8

	err = json.NewEncoder(tmpFile).Encode(conf)
	Ω(err).ShouldNot(HaveOccured())
}

func (runner *CLIRunner) StartListener(timestamp int) {
	runner.start("listen", timestamp)
}

func (runner *CLIRunner) StopListener() {
	runner.listenerCmd.Process.Kill()
}

func (runner *CLIRunner) Cleanup() {
	os.Remove(runner.configPath)
}

func (runner *CLIRunner) start(command string, timestamp int) {
	runner.listenerCmd = exec.Command("hm9000", command, fmt.Sprintf("--config=%s", runner.configPath))
	runner.listenerCmd.Env = append(os.Environ(), fmt.Sprintf("HM9000_FAKE_TIME=%d", timestamp))
	runner.listenerStdoutBuffer = bytes.NewBuffer([]byte{})
	runner.listenerCmd.Stdout = runner.listenerStdoutBuffer
	runner.listenerCmd.Start()
	Eventually(func() int {
		return runner.listenerStdoutBuffer.Len()
	}).ShouldNot(BeZero())
}

func (runner *CLIRunner) WaitForHeartbeats(num int) {
	Eventually(func() int {
		var validHeartbeat = regexp.MustCompile(`Received dea.heartbeat`)
		heartbeats := validHeartbeat.FindAll(runner.listenerStdoutBuffer.Bytes(), -1)
		return len(heartbeats)
	}).Should(BeNumerically("==", num))
}

func (runner *CLIRunner) Run(command string, timestamp int) {
	cmd := exec.Command("hm9000", command, fmt.Sprintf("--config=%s", runner.configPath))
	cmd.Env = append(os.Environ(), fmt.Sprintf("HM9000_FAKE_TIME=%d", timestamp))
	out, err := cmd.CombinedOutput()
	Ω(err).ShouldNot(HaveOccured(), "%s command failed", command)
	if runner.verbose {
		fmt.Printf(command + "\n")
		fmt.Printf(strings.Repeat("~", len(command)) + "\n")
		fmt.Printf(string(out))
		fmt.Printf("\n")
	}
	time.Sleep(50 * time.Millisecond) //give NATS a chance to send messages around, if necessary
}
