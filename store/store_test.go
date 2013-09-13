package store

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"

	"testing"
)

var runner *etcd_runner.ETCDRunner
var etcdPort int

func TestBootstrap(t *testing.T) {
	RegisterFailHandler(Fail)

	etcdPort = 4000 + config.GinkgoConfig.ParallelNode
	runner = etcd_runner.NewETCDRunner("etcd", etcdPort)
	runner.StartETCD()

	RunSpecs(t, "Store tests")

	runner.StopETCD()
}

var _ = BeforeEach(func() {
	runner.Reset()
})
