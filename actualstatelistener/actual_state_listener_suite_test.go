package actualstatelistener_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"

	"testing"
)

var etcdRunner *storerunner.ETCDClusterRunner

func TestActualStateListener(t *testing.T) {
	RegisterFailHandler(Fail)

	etcdRunner = storerunner.NewETCDClusterRunner(5001, 1)
	etcdRunner.Start()

	RunSpecs(t, "Actual State Listener Suite")

	etcdRunner.Stop()
}

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})
