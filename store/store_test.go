package store

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"

	"testing"
)

var runner *etcd_runner.ETCDClusterRunner
var etcdPort int

func TestBootstrap(t *testing.T) {
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
