package shredder_test

import (
	"github.com/cloudfoundry/hm9000/config"
	. "github.com/cloudfoundry/hm9000/shredder"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Shredder", func() {
	var (
		shredder     *Shredder
		storeAdapter *fakestoreadapter.FakeStoreAdapter
	)

	BeforeEach(func() {
		storeAdapter = fakestoreadapter.New()
		conf, _ := config.DefaultConfig()
		store := storepackage.NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
		shredder = New(store)

		storeAdapter.Set([]storeadapter.StoreNode{
			{Key: "/v1/pokemon/geodude", Value: []byte{}},
			{Key: "/v1/deep-pokemon/abra/kadabra/alakazam", Value: []byte{}},
			{Key: "/v1/pokemonCount", Value: []byte("151")},
			{Key: "/v1/dea-presence/ABC", Value: []byte("ABC")},
			{Key: "/v1/dea-summary/ABC", Value: []byte("summary...")},
			{Key: "/v1/dea-summary/DEF", Value: []byte("summary...")},
		})

		storeAdapter.Delete("/v1/pokemon/geodude", "/v1/deep-pokemon/abra/kadabra/alakazam")
		err := shredder.Shred()
		Ω(err).ShouldNot(HaveOccured())
	})

	It("should delete empty directories", func() {
		_, err := storeAdapter.Get("/v1/pokemon")
		Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

		_, err = storeAdapter.Get("/v1/deep-pokemon")
		Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

		_, err = storeAdapter.Get("/v1/pokemonCount")
		Ω(err).ShouldNot(HaveOccured())
	})

	It("should delete expired dea summaries", func() {
		_, err := storeAdapter.Get("/v1/dea-summary/DEF")
		Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

		_, err = storeAdapter.Get("/v1/dea-summary/ABC")
		Ω(err).ShouldNot(HaveOccured())
	})
})
