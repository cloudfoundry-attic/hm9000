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
	etcdRunner = etcdstorerunner.NewETCDClusterRunner(5001, 1, nil)
	etcdRunner.Start()
	return nil
}, func([]byte) {

})

var _ = SynchronizedAfterSuite(func() {
	etcdRunner.Stop()
}, func() {

})

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})
