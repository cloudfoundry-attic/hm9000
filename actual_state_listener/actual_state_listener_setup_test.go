package actual_state_listener

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"
	"github.com/cloudfoundry/hm9000/test_helpers/message_publisher"
	"github.com/cloudfoundry/hm9000/test_helpers/nats_runner"

	"os"
	"os/signal"

	"testing"
)

var etcdRunner *etcd_runner.ETCDRunner
var natsRunner *nats_runner.NATSRunner
var messagePublisher *message_publisher.MessagePublisher

func TestBootstrap(t *testing.T) {
	RegisterFailHandler(Fail)

	natsRunner = nats_runner.NewNATSRunner(4222)
	natsRunner.Start()

	etcdRunner = etcd_runner.NewETCDRunner("etcd")
	messagePublisher = message_publisher.NewMessagePublisher(natsRunner.MessageBus)

	RunSpecs(t, "Actual State Listener Tests")

	etcdRunner.StopETCD()
	natsRunner.Stop()
}

var _ = BeforeEach(func() {
	natsRunner.MessageBus.UnsubscribeAll()
	etcdRunner.StopETCD()
	etcdRunner.StartETCD()
})

func registerSignalHandler() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		select {
		case <-c:
			etcdRunner.StopETCD()
			natsRunner.Stop()
			os.Exit(0)
		}
	}()
}
