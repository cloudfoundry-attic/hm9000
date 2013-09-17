package actualstatelistener

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"

	"testing"
)

var etcdRunner *etcd_runner.ETCDClusterRunner

func TestBootstrap(t *testing.T) {
	RegisterFailHandler(Fail)

	etcdRunner = etcd_runner.NewETCDClusterRunner("etcd", 5001, 1)
	etcdRunner.Start()

	RunSpecs(t, "Actual State Listener Tests")

	etcdRunner.Stop()
}

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})
