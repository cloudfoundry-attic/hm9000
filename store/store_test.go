package store

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"

	"os"
	"os/signal"
	"testing"
)

var runner *etcd_runner.ETCDClusterRunner
var etcdPort int

func TestBootstrap(t *testing.T) {
	registerSignalHandler()
	RegisterFailHandler(Fail)

	etcdPort = 5000 + config.GinkgoConfig.ParallelNode*10
	runner = etcd_runner.NewETCDClusterRunner("etcd", etcdPort, 5)
	runner.Start()

	RunSpecs(t, "Store tests")

	runner.Stop()
}

var _ = BeforeEach(func() {
	runner.Reset()
})

func registerSignalHandler() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		select {
		case <-c:
			runner.Stop()
			os.Exit(0)
		}
	}()
}
