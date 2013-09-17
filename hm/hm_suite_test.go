package hm

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"

	"testing"
)

var etcdRunner *etcd_runner.ETCDRunner

func TestHm9000(t *testing.T) {
	RegisterFailHandler(Fail)

	etcdRunner = etcd_runner.NewETCDRunner("etcd", 4001)
	etcdRunner.StartETCD()

	RunSpecs(t, "Hm9000 Suite")

	etcdRunner.StopETCD()
}

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})
