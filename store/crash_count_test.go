package store_test

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CrashCount", func() {
	var (
		store       Store
		etcdAdapter storeadapter.StoreAdapter
		conf        config.Config
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())
		etcdAdapter = storeadapter.NewETCDStoreAdapter(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err = etcdAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())

		store = NewStore(conf, etcdAdapter, fakelogger.NewFakeLogger())
	})

	Describe("storing a crash count", func() {
		It("should allow to save, get and delete crash counts", func() {
			crashCount := models.CrashCount{
				AppGuid:       "abc",
				AppVersion:    "xyz",
				InstanceIndex: 1,
				CrashCount:    12,
			}
			err := store.SaveCrashCounts(crashCount)
			Ω(err).ShouldNot(HaveOccured())

			node, err := etcdAdapter.Get("/crashes/abc-xyz-1")
			Ω(err).ShouldNot(HaveOccured())
			Ω(node.TTL).Should(BeNumerically("==", 1919))

			results, err := store.GetCrashCounts()
			Ω(err).ShouldNot(HaveOccured())
			Ω(results).Should(HaveLen(1))
			Ω(results).Should(ContainElement(crashCount))

			err = store.DeleteCrashCounts(results["abc-xyz-1"])
			Ω(err).ShouldNot(HaveOccured())

			results, err = store.GetCrashCounts()
			Ω(err).ShouldNot(HaveOccured())
			Ω(results).Should(BeEmpty())
		})
	})
})
