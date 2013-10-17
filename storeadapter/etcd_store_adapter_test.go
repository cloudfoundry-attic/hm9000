package storeadapter_test

import (
	. "github.com/cloudfoundry/hm9000/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ETCD Store Adapter", func() {
	var (
		adapter       StoreAdapter
		breakfastNode StoreNode
		lunchNode     StoreNode
	)

	BeforeEach(func() {
		breakfastNode = StoreNode{
			Key:   "/menu/breakfast",
			Value: []byte("waffles"),
		}

		lunchNode = StoreNode{
			Key:   "/menu/lunch",
			Value: []byte("burgers"),
		}

		adapter = NewETCDStoreAdapter(etcdRunner.NodeURLS(), 100)
		err := adapter.Connect()
		Ω(err).ShouldNot(HaveOccured())
	})

	AfterEach(func() {
		adapter.Disconnect()
	})

	Describe("Get", func() {
		BeforeEach(func() {
			err := adapter.Set([]StoreNode{breakfastNode, lunchNode})
			Ω(err).ShouldNot(HaveOccured())
		})

		Context("when getting a key", func() {
			It("should return the appropriate store breakfastNode", func() {
				value, err := adapter.Get("/menu/breakfast")
				Ω(err).ShouldNot(HaveOccured())
				Ω(value).Should(Equal(breakfastNode))
			})
		})

		Context("When getting a non-existent key", func() {
			It("should return an error", func() {
				value, err := adapter.Get("/not_a_key")
				Ω(err).Should(Equal(ErrorKeyNotFound))
				Ω(value).Should(BeZero())
			})
		})

		Context("when getting a directory", func() {
			It("should return an error", func() {
				value, err := adapter.Get("/menu")
				Ω(err).Should(Equal(ErrorNodeIsDirectory))
				Ω(value).Should(BeZero())
			})
		})

		Context("when the store is down", func() {
			BeforeEach(func() {
				etcdRunner.Stop()
			})

			AfterEach(func() {
				etcdRunner.Start()
			})

			It("should return a timeout error", func() {
				value, err := adapter.Get("/foo/bar")
				Ω(err).Should(Equal(ErrorTimeout))
				Ω(value).Should(BeZero())
			})
		})
	})

	Describe("Set", func() {
		It("should be able to set multiple things to the store at once", func() {
			err := adapter.Set([]StoreNode{breakfastNode, lunchNode})
			Ω(err).ShouldNot(HaveOccured())

			values, err := adapter.List("/menu")
			Ω(err).ShouldNot(HaveOccured())
			Ω(values).Should(HaveLen(2))
			Ω(values).Should(ContainElement(breakfastNode))
			Ω(values).Should(ContainElement(lunchNode))
		})

		Context("Setting to an existing node", func() {
			BeforeEach(func() {
				err := adapter.Set([]StoreNode{breakfastNode, lunchNode})
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should be able to update existing entries", func() {
				lunchNode.Value = []byte("steak")
				err := adapter.Set([]StoreNode{breakfastNode, lunchNode})
				Ω(err).ShouldNot(HaveOccured())

				values, err := adapter.List("/menu")
				Ω(err).ShouldNot(HaveOccured())
				Ω(values).Should(HaveLen(2))
				Ω(values).Should(ContainElement(breakfastNode))
				Ω(values).Should(ContainElement(lunchNode))
			})

			It("should error when attempting to set to a directory", func() {
				dirNode := StoreNode{
					Key:   "/menu",
					Value: []byte("oops!"),
				}

				err := adapter.Set([]StoreNode{dirNode})
				Ω(err).Should(Equal(ErrorNodeIsDirectory))
			})
		})

		Context("when the store is down", func() {
			BeforeEach(func() {
				etcdRunner.Stop()
			})

			AfterEach(func() {
				etcdRunner.Start()
			})

			It("should return a timeout error", func() {
				err := adapter.Set([]StoreNode{breakfastNode})
				Ω(err).Should(Equal(ErrorTimeout))
			})
		})
	})

	Describe("List", func() {
		BeforeEach(func() {
			err := adapter.Set([]StoreNode{breakfastNode, lunchNode})
			Ω(err).ShouldNot(HaveOccured())
		})

		Context("When listing a directory", func() {
			It("Should list directory contents", func() {
				values, err := adapter.List("/menu")
				Ω(err).ShouldNot(HaveOccured())
				Ω(values).Should(HaveLen(2))
				Ω(values).Should(ContainElement(breakfastNode))
				Ω(values).Should(ContainElement(lunchNode))
			})
		})

		Context("when listing an empty directory", func() {
			It("should return an empty list of breakfastNodes, and not error", func() {
				err := adapter.Delete("/menu/breakfast")
				Ω(err).ShouldNot(HaveOccured())
				err = adapter.Delete("/menu/lunch")
				Ω(err).ShouldNot(HaveOccured())

				values, err := adapter.List("/menu")
				Ω(err).ShouldNot(HaveOccured())
				Ω(values).Should(HaveLen(0))
			})
		})

		Context("when listing a non-existent key", func() {
			It("should return an error", func() {
				values, err := adapter.List("/nothing-here")
				Ω(err).Should(Equal(ErrorKeyNotFound))
				Ω(values).Should(BeEmpty())
			})
		})

		Context("When listing an entry", func() {
			It("should return an error", func() {
				_, err := adapter.List("/menu/breakfast")
				Ω(err).Should(HaveOccured())
				Ω(err).Should(Equal(ErrorNodeIsNotDirectory))
			})
		})

		Context("when the store is down", func() {
			BeforeEach(func() {
				etcdRunner.Stop()
			})

			AfterEach(func() {
				etcdRunner.Start()
			})

			It("should return a timeout error", func() {
				_, err := adapter.List("/menu")
				Ω(err).Should(Equal(ErrorTimeout))
			})
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			err := adapter.Set([]StoreNode{breakfastNode})
			Ω(err).ShouldNot(HaveOccured())
		})

		Context("when deleting an existing key", func() {
			It("should delete the key", func() {
				err := adapter.Delete("/menu/breakfast")
				Ω(err).ShouldNot(HaveOccured())

				value, err := adapter.Get("/menu/breakfat")
				Ω(err).Should(Equal(ErrorKeyNotFound))
				Ω(value).Should(BeZero())
			})
		})

		Context("when deleting a non-existing key", func() {
			It("should error", func() {
				err := adapter.Delete("/not-a-key")
				Ω(err).Should(Equal(ErrorKeyNotFound))
			})
		})

		Context("when the store is down", func() {
			BeforeEach(func() {
				etcdRunner.Stop()
			})

			AfterEach(func() {
				etcdRunner.Start()
			})

			It("should return a timeout error", func() {
				err := adapter.Delete("/menu/breakfast")
				Ω(err).Should(Equal(ErrorTimeout))
			})
		})
	})

	Context("When setting a key with a non-zero TTL", func() {
		It("should stay in the adapter for its TTL and then disappear", func() {
			breakfastNode.TTL = 1
			err := adapter.Set([]StoreNode{breakfastNode})
			Ω(err).ShouldNot(HaveOccured())

			_, err = adapter.Get("/menu/breakfast")
			Ω(err).ShouldNot(HaveOccured())

			Eventually(func() interface{} {
				_, err = adapter.Get("/menu/breakfast")
				return err
			}, 1.05, 0.01).Should(Equal(ErrorKeyNotFound))
		})
	})
})
