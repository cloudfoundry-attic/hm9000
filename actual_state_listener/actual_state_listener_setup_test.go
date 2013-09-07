package actual_state_listener

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/mcat/message_publisher"
	"github.com/cloudfoundry/hm9000/mcat/nats_runner"
	"github.com/cloudfoundry/hm9000/store"

	"os"
	"os/signal"

	"testing"
)

var etcdRunner *store.ETCDRunner
var natsRunner *nats_runner.NATSRunner
var messagePublisher *message_publisher.MessagePublisher

func TestBootstrap(t *testing.T) {
	RegisterFailHandler(Fail)

	natsRunner = nats_runner.NewNATSRunner(4222)
	natsRunner.Start()

	etcdRunner = store.NewETCDRunner("etcd")
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
