package shredder_test

import (
	. "github.com/cloudfoundry/hm9000/shredder"
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
		shredder = New(storeAdapter, fakelogger.NewFakeLogger())

		storeAdapter.Set([]storeadapter.StoreNode{
			{Key: "/pokemon/geodude", Value: []byte{}},
			{Key: "/deep-pokemon/abra/kadabra/alakazam", Value: []byte{}},
			{Key: "/pokemonCount", Value: []byte("151")},
		})
	})

	Describe("recursing through the store", func() {
		Context("when the node is a directory", func() {
			Context("and it is empty", func() {
				BeforeEach(func() {
					storeAdapter.Delete("/pokemon/geodude")
				})

				It("shreds it mercilessly", func() {
					err := shredder.Shred()
					Ω(err).ShouldNot(HaveOccured())

					_, err = storeAdapter.Get("/pokemon")
					Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))
				})
			})

			Context("and it is non-empty", func() {
				It("spares it", func() {
					err := shredder.Shred()
					Ω(err).ShouldNot(HaveOccured())

					_, err = storeAdapter.Get("/pokemon/geodude")
					Ω(err).ShouldNot(HaveOccured())
				})

				Context("but all of its children are empty", func() {
					BeforeEach(func() {
						storeAdapter.Delete("/deep-pokemon/abra/kadabra/alakazam")
					})

					It("shreds it mercilessly", func() {
						err := shredder.Shred()
						Ω(err).ShouldNot(HaveOccured())

						_, err = storeAdapter.Get("/deep-pokemon/abra/kadabra")
						Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

						_, err = storeAdapter.Get("/deep-pokemon/abra")
						Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))
					})
				})
			})
		})

		Context("when the node is NOT a directory", func() {
			It("spares it", func() {
				err := shredder.Shred()
				Ω(err).ShouldNot(HaveOccured())

				_, err = storeAdapter.Get("/pokemonCount")
				Ω(err).ShouldNot(HaveOccured())
			})
		})
	})
})
