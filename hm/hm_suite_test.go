package hm

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"

	"testing"
)

var etcdRunner *etcd_runner.ETCDClusterRunner

func TestHm9000(t *testing.T) {
	RegisterFailHandler(Fail)

	etcdRunner = etcd_runner.NewETCDClusterRunner("etcd", 5001, 1)
	etcdRunner.Start()

	RunSpecs(t, "Hm9000 Suite")

	etcdRunner.Stop()
}

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})
