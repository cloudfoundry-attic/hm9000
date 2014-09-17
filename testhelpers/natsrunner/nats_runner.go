package natsrunner

import (
	"fmt"
	"os"

	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/yagnats"

	"os/exec"
	"strconv"
)

var natsCommand *exec.Cmd

type NATSRunner struct {
	port        int
	natsCommand *exec.Cmd
	MessageBus  yagnats.ApceraWrapperNATSClient
}

func NewNATSRunner(port int) *NATSRunner {
	return &NATSRunner{
		port: port,
	}
}

func (runner *NATSRunner) Start() {
	_, err := exec.LookPath("gnatsd")
	if err != nil {
		fmt.Println("You need gnatsd installed!")
		os.Exit(1)
	}

	runner.natsCommand = exec.Command("gnatsd", "-p", strconv.Itoa(runner.port))
	err = runner.natsCommand.Start()
	Ω(err).ShouldNot(HaveOccurred(), "Make sure to have gnatsd on your path")

	messageBus := yagnats.NewApceraClientWrapper([]string{fmt.Sprintf("nats://127.0.0.1:%d", runner.port)})

	Eventually(func() error {
		return messageBus.Connect()
	}, 5, 0.1).ShouldNot(HaveOccurred())

	runner.MessageBus = messageBus
}

func (runner *NATSRunner) Stop() {
	if runner.natsCommand != nil {
		runner.natsCommand.Process.Kill()
		runner.MessageBus = nil
		runner.natsCommand = nil
	}
}
