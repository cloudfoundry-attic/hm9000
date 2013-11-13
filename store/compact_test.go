package store_test

import (
	"github.com/cloudfoundry/hm9000/helpers/workerpool"
	. "github.com/cloudfoundry/hm9000/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
)

var _ = Describe("Compact", func() {
	var (
		store        Store
		storeAdapter storeadapter.StoreAdapter
		conf         config.Config
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())
		storeAdapter = storeadapter.NewETCDStoreAdapter(etcdRunner.NodeURLS(), workerpool.NewWorkerPool(conf.StoreMaxConcurrentRequests))
		err = storeAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())
		store = NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
	})

	Describe("Removing expired DEA heartbeat summaries", func() {
		var dea1, dea2 appfixture.DeaFixture
		BeforeEach(func() {
			dea1 = appfixture.NewDeaFixture()
			dea2 = appfixture.NewDeaFixture()
			store.SyncHeartbeat(dea1.HeartbeatWith(dea1.GetApp(0).InstanceAtIndex(0).Heartbeat()))
			store.SyncHeartbeat(dea2.HeartbeatWith(dea2.GetApp(0).InstanceAtIndex(0).Heartbeat()))

			storeAdapter.Delete("/v1/dea-presence/" + dea1.DeaGuid)
			err := store.Compact()

			Ω(err).ShouldNot(HaveOccured())
		})

		It("should remove DEA summaries that have expired", func() {
			_, err := storeAdapter.Get("/v1/dea-summary/" + dea1.DeaGuid)
			Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

			_, err = storeAdapter.Get("/v1/dea-summary/" + dea2.DeaGuid)
			Ω(err).ShouldNot(HaveOccured())
		})
	})

	Describe("Recursively deleting empty directories", func() {
		BeforeEach(func() {
			storeAdapter.Set([]storeadapter.StoreNode{
				{Key: "/v1/pokemon/geodude", Value: []byte("foo")},
				{Key: "/v1/deep-pokemon/abra/kadabra/alakazam", Value: []byte{}},
				{Key: "/v1/pokemonCount", Value: []byte("151")},
			})
		})

		Context("when the node is a directory", func() {
			Context("and it is empty", func() {
				BeforeEach(func() {
					storeAdapter.Delete("/v1/pokemon/geodude")
				})

				It("shreds it mercilessly", func() {
					err := store.Compact()
					Ω(err).ShouldNot(HaveOccured())

					_, err = storeAdapter.Get("/v1/pokemon")
					Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))
				})
			})

			Context("and it is non-empty", func() {
				It("spares it", func() {
					err := store.Compact()
					Ω(err).ShouldNot(HaveOccured())

					_, err = storeAdapter.Get("/v1/pokemon/geodude")
					Ω(err).ShouldNot(HaveOccured())
				})

				Context("but all of its children are empty", func() {
					BeforeEach(func() {
						storeAdapter.Delete("/v1/deep-pokemon/abra/kadabra/alakazam")
					})

					It("shreds it mercilessly", func() {
						err := store.Compact()
						Ω(err).ShouldNot(HaveOccured())

						_, err = storeAdapter.Get("/v1/deep-pokemon/abra/kadabra")
						Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

						_, err = storeAdapter.Get("/v1/deep-pokemon/abra")
						Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))
					})
				})
			})
		})

		Context("when the node is NOT a directory", func() {
			It("spares it", func() {
				err := store.Compact()
				Ω(err).ShouldNot(HaveOccured())

				_, err = storeAdapter.Get("/v1/pokemonCount")
				Ω(err).ShouldNot(HaveOccured())
			})
		})

	})
})
