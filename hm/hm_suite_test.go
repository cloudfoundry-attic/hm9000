package hm_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"

	"testing"
)

var etcdRunner *etcdstorerunner.ETCDClusterRunner

func TestHM9000(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HM9000 CLI Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	return nil
}, func([]byte) {
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(5001+10*GinkgoParallelNode(), 1, nil)
	etcdRunner.Start()
})

var _ = SynchronizedAfterSuite(func() {
	etcdRunner.Stop()
}, func() {

})

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})
