package phd_aws

import (
	"github.com/cloudfoundry/hm9000/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var storeAdapter storeadapter.StoreAdapter

func TestPhdAWS(t *testing.T) {
	RegisterFailHandler(Fail)

	storeAdapter = storeadapter.NewETCDStoreAdapter([]string{"http://127.0.0.1:4001"}, 10)
	err := storeAdapter.Connect()
	Ω(err).ShouldNot(HaveOccured())

	RunSpecsWithDefaultAndCustomReporters(t, "MCAT AWS PhD Suite", []Reporter{&DataReporter{Title: "Local_ETCD"}})

	storeAdapter.Disconnect()
}
