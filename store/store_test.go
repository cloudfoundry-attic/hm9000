package store

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var runner *ETCDRunner

func TestBootstrap(t *testing.T) {
	RegisterFailHandler(Fail)

	runner = NewETCDRunner("etcd")

	RunSpecs(t, "Store tests")

	runner.StopETCD()
}

var _ = BeforeEach(func() {
	runner.StopETCD()
})
