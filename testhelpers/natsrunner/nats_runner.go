package natsrunner

import (
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/go_cfmessagebus"

	"os/exec"
	"strconv"
)

var natsCommand *exec.Cmd

type NATSRunner struct {
	port        int
	natsCommand *exec.Cmd
	MessageBus  cfmessagebus.MessageBus
}

func NewNATSRunner(port int) *NATSRunner {
	return &NATSRunner{
		port: port,
	}
}

func (runner *NATSRunner) Start() {
	runner.natsCommand = exec.Command("nats-server", "-p", strconv.Itoa(runner.port))
	err := runner.natsCommand.Start()
	Ω(err).ShouldNot(HaveOccured(), "Make sure to have nats-server on your path")

	messageBus, err := cfmessagebus.NewMessageBus("NATS")
	Ω(err).ShouldNot(HaveOccured())
	messageBus.Configure("127.0.0.1", runner.port, "", "")

	Eventually(func() error {
		return messageBus.Connect()
	}, 5, 0.1).ShouldNot(HaveOccured())

	runner.MessageBus = messageBus
}

func (runner *NATSRunner) Stop() {
	if runner.natsCommand != nil {
		runner.natsCommand.Process.Kill()
		runner.MessageBus = nil
		runner.natsCommand = nil
	}
}
