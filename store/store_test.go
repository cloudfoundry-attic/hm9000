package store

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"

	"testing"
)

var runner *etcd_runner.ETCDRunner

func TestBootstrap(t *testing.T) {
	RegisterFailHandler(Fail)

	runner = etcd_runner.NewETCDRunner("etcd")

	RunSpecs(t, "Store tests")

	runner.StopETCD()
}

var _ = BeforeEach(func() {
	runner.StopETCD()
})
