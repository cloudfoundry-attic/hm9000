package store_test

import (
	"code.cloudfoundry.org/workpool"
	. "github.com/cloudfoundry/hm9000/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
)

var _ = Describe("Compact", func() {
	var (
		store        Store
		storeAdapter storeadapter.StoreAdapter
		conf         *config.Config
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		conf.StoreSchemaVersion = 17
		Expect(err).NotTo(HaveOccurred())
		wpool, err := workpool.NewWorkPool(conf.StoreMaxConcurrentRequests)
		Expect(err).NotTo(HaveOccurred())
		storeAdapter, err = etcdstoreadapter.New(
			&etcdstoreadapter.ETCDOptions{ClusterUrls: etcdRunner.NodeURLS()},
			wpool,
		)
		Expect(err).NotTo(HaveOccurred())
		err = storeAdapter.Connect()
		Expect(err).NotTo(HaveOccurred())
		store = NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
	})

	Describe("Deleting old schema version", func() {
		BeforeEach(func() {
			storeAdapter.SetMulti([]storeadapter.StoreNode{
				{Key: "/hm/v3/delete/me", Value: []byte("abc")},
				{Key: "/hm/v16/delete/me", Value: []byte("abc")},
				{Key: "/hm/v17/leave/me/alone", Value: []byte("abc")},
				{Key: "/hm/v17/leave/me/v1/alone", Value: []byte("abc")},
				{Key: "/hm/v18/leave/me/alone", Value: []byte("abc")},
				{Key: "/hm/delete/me", Value: []byte("abc")},
				{Key: "/hm/v1ola/delete/me", Value: []byte("abc")},
				{Key: "/hm/delete/me/too", Value: []byte("abc")},
				{Key: "/hm/locks/keep", Value: []byte("abc")},
				{Key: "/other/keep", Value: []byte("abc")},
				{Key: "/foo", Value: []byte("abc")},
				{Key: "/v3/keep", Value: []byte("abc")},
			})

			err := store.Compact()
			Expect(err).NotTo(HaveOccurred())
		})

		It("To delete everything under older versions", func() {
			_, err := storeAdapter.Get("/hm/v3/delete/me")
			Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))

			_, err = storeAdapter.Get("/hm/v16/delete/me")
			Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
		})

		It("To leave the current version alone", func() {
			_, err := storeAdapter.Get("/hm/v17/leave/me/alone")
			Expect(err).NotTo(HaveOccurred())

			_, err = storeAdapter.Get("/hm/v17/leave/me/v1/alone")
			Expect(err).NotTo(HaveOccurred())
		})

		It("To leave newer versions alone", func() {
			_, err := storeAdapter.Get("/hm/v18/leave/me/alone")
			Expect(err).NotTo(HaveOccurred())
		})

		It("To leave locks alone", func() {
			_, err := storeAdapter.Get("/hm/locks/keep")
			Expect(err).NotTo(HaveOccurred())
		})

		It("To delete anything that's unversioned", func() {
			_, err := storeAdapter.Get("/hm/delete/me")
			Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))

			_, err = storeAdapter.Get("/hm/v1ola/delete/me")
			Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))

			_, err = storeAdapter.Get("/hm/delete/me/too")
			Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
		})

		It("To not touch anything that isn't under the hm namespace", func() {
			_, err := storeAdapter.Get("/other/keep")
			Expect(err).NotTo(HaveOccurred())

			_, err = storeAdapter.Get("/foo")
			Expect(err).NotTo(HaveOccurred())

			_, err = storeAdapter.Get("/v3/keep")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Recursively deleting empty directories", func() {
		BeforeEach(func() {
			storeAdapter.SetMulti([]storeadapter.StoreNode{
				{Key: "/hm/v17/pokemon/geodude", Value: []byte("foo")},
				{Key: "/hm/v17/deep-pokemon/abra/kadabra/alakazam", Value: []byte{}},
				{Key: "/hm/v17/pokemonCount", Value: []byte("151")},
			})
		})

		Context("when the node is a directory", func() {
			Context("and it is empty", func() {
				BeforeEach(func() {
					storeAdapter.Delete("/hm/v17/pokemon/geodude")
				})

				It("shreds it mercilessly", func() {
					err := store.Compact()
					Expect(err).NotTo(HaveOccurred())

					_, err = storeAdapter.Get("/hm/v17/pokemon")
					Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
				})
			})

			Context("and it is non-empty", func() {
				It("spares it", func() {
					err := store.Compact()
					Expect(err).NotTo(HaveOccurred())

					_, err = storeAdapter.Get("/hm/v17/pokemon/geodude")
					Expect(err).NotTo(HaveOccurred())
				})

				Context("but all of its children are empty", func() {
					BeforeEach(func() {
						storeAdapter.Delete("/hm/v17/deep-pokemon/abra/kadabra/alakazam")
					})

					It("shreds it mercilessly", func() {
						err := store.Compact()
						Expect(err).NotTo(HaveOccurred())

						_, err = storeAdapter.Get("/hm/v17/deep-pokemon/abra/kadabra")
						Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))

						_, err = storeAdapter.Get("/hm/v17/deep-pokemon/abra")
						Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
					})
				})
			})
		})

		Context("when the node is NOT a directory", func() {
			It("spares it", func() {
				err := store.Compact()
				Expect(err).NotTo(HaveOccurred())

				_, err = storeAdapter.Get("/hm/v17/pokemonCount")
				Expect(err).NotTo(HaveOccurred())
			})
		})

	})
})
