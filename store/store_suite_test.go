package store_test

import (
	"os"
	"os/signal"
	"testing"

	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
)

var (
	etcdRunner  *etcdstorerunner.ETCDClusterRunner
)

func TestStore(t *testing.T) {
	registerSignalHandler()
	RegisterFailHandler(Fail)

	RunSpecs(t, "Store Suite")
}

var _ = BeforeSuite(func() {
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(5001+config.GinkgoConfig.ParallelNode, 1, nil)
	etcdRunner.Start()
	Expect(len(etcdRunner.NodeURLS())).Should(BeNumerically(">=", 1))
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})

var _ = AfterSuite(func() {
	etcdRunner.Stop()
})

func registerSignalHandler() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		select {
		case <-c:
			etcdRunner.Stop()
			os.Exit(0)
		}
	}()
}
