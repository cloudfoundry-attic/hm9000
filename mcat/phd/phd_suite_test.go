package phd

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"
	"github.com/cloudfoundry/hm9000/test_helpers/nats_runner"

	"os"
	"os/signal"
	"testing"
)

var etcdRunner *etcd_runner.ETCDClusterRunner
var natsRunner *nats_runner.NATSRunner

func TestBootstrap(t *testing.T) {
	registerSignalHandler()
	RegisterFailHandler(Fail)

	natsRunner = nats_runner.NewNATSRunner(4223)
	natsRunner.Start()

	etcdRunner = etcd_runner.NewETCDClusterRunner("etcd", 5001, 1)
	etcdRunner.Start()

	RunSpecs(t, "Phd Suite")

	natsRunner.Stop()
	etcdRunner.Stop()
}

var _ = BeforeEach(func() {
	natsRunner.MessageBus.UnsubscribeAll()
	etcdRunner.Reset()
})

func registerSignalHandler() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		select {
		case <-c:
			etcdRunner.Stop()
			natsRunner.Stop()
			os.Exit(0)
		}
	}()
}
