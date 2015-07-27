package store_test

import (
	"io/ioutil"
	"net/http"
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
	etcdVersion = "2.1.1"
)

func TestStore(t *testing.T) {
	registerSignalHandler()
	RegisterFailHandler(Fail)

	etcdRunner = etcdstorerunner.NewETCDClusterRunner(5001+config.GinkgoConfig.ParallelNode, 1)

	etcdRunner.Start()
	RunSpecs(t, "Store Suite")
	etcdRunner.Stop()
}

var _ = BeforeSuite(func() {
	Expect(len(etcdRunner.NodeURLS())).Should(BeNumerically(">=", 1))

	etcdVersionUrl := etcdRunner.NodeURLS()[0] + "/version"
	resp, err := http.Get(etcdVersionUrl)
	Expect(err).ToNot(HaveOccurred())

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	Expect(err).ToNot(HaveOccurred())

	Expect(string(body)).To(ContainSubstring(etcdVersion))
})

var _ = BeforeEach(func() {
	etcdRunner.Reset()
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
