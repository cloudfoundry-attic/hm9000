package store

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/config"
)

var _ = Describe("ETCD Store", func() {
	var store Store
	BeforeEach(func() {
		store = NewETCDStore(config.ETCD_URL(etcdPort))
		err := store.Connect()
		Ω(err).ShouldNot(HaveOccured())
	})

	Context("With something in the store", func() {
		var key string
		var value []byte
		var dir_key string
		var dir_entry_key string

		var expectedLeafNode StoreNode
		var expectedDirNode StoreNode

		BeforeEach(func() {
			value = []byte("my_stuff")

			key = "/foo/bar"
			err := store.Set(key, value, 0)
			Ω(err).ShouldNot(HaveOccured())

			dir_key = "/foo/baz"
			dir_entry_key = "/bar"
			err = store.Set(dir_key+dir_entry_key, value, 0)
			Ω(err).ShouldNot(HaveOccured())

			expectedLeafNode = StoreNode{
				Key:   key,
				Value: value,
				Dir:   false,
				TTL:   0,
			}

			expectedDirNode = StoreNode{
				Key:   dir_key,
				Value: []byte(""),
				Dir:   true,
				TTL:   0,
			}
		})

		It("should be able to set and get things from the store", func() {
			value, err := store.Get("/foo/bar")
			Ω(err).ShouldNot(HaveOccured())
			Ω(value).Should(Equal(expectedLeafNode))
		})

		It("Should list directory contents", func() {
			value, err := store.List("/foo")
			Ω(err).ShouldNot(HaveOccured())
			Ω(value).Should(HaveLen(2))
			Ω(value).Should(ContainElement(expectedLeafNode))
			Ω(value).Should(ContainElement(expectedDirNode))
		})

		It("should be able to delete things from the store", func() {
			err := store.Delete("/foo/bar")
			_, err = store.Get("/foo/bar")
			Ω(err).Should(HaveOccured())
			Ω(IsKeyNotFoundError(err)).Should(BeTrue())
		})

		Context("when we call List on an entry", func() {
			It("should return an error", func() {
				_, err := store.List(key)
				Ω(err).Should(HaveOccured())
				Ω(IsNotDirectoryError(err)).Should(BeTrue())
			})
		})

		Context("when we call Get on a directory", func() {
			It("should return an error", func() {
				_, err := store.Get(dir_key)
				Ω(err).Should(HaveOccured())
				Ω(IsDirectoryError(err)).Should(BeTrue())
			})
		})

		Context("when listing an empty directory", func() {
			It("should return an empty list of nodes and no error", func() {
				store.Set("/menu/waffles", []byte("tasty"), 0)
				store.Delete("/menu/waffles")
				results, err := store.List("/menu")
				Ω(results).Should(BeEmpty())
				Ω(err).ShouldNot(HaveOccured())
			})
		})
	})

	Context("when the store is down", func() {
		BeforeEach(func() {
			runner.StopETCD()
		})

		AfterEach(func() {
			runner.StartETCD()
		})

		Context("when we get", func() {
			It("should return a timeout error", func() {
				_, err := store.Get("/foo/bar")
				Ω(IsTimeoutError(err)).Should(BeTrue())
			})
		})

		Context("when we set", func() {
			It("should return a timeout error", func() {
				err := store.Set("/foo/bar", []byte("baz"), 0)
				Ω(IsTimeoutError(err)).Should(BeTrue())
			})
		})

		Context("when we list", func() {
			It("should return a timeout error", func() {
				_, err := store.List("/foo/bar")
				Ω(IsTimeoutError(err)).Should(BeTrue())
			})
		})

		Context("when we delete", func() {
			It("should return a timeout error", func() {
				err := store.Delete("/foo/bar")
				Ω(IsTimeoutError(err)).Should(BeTrue())
			})
		})
	})

	Context("When fetching a non-existent key", func() {
		It("should return an error", func() {
			_, err := store.Get("/not_a_key")
			Ω(err).Should(HaveOccured())
			Ω(IsKeyNotFoundError(err)).Should(BeTrue())
		})
	})

	Context("When setting a key with a non-zero TTL", func() {
		It("should stay in the store for its TTL and then disappear", func() {
			err := store.Set("/floop", []byte("bar"), 1)
			Ω(err).ShouldNot(HaveOccured())

			_, err = store.Get("/floop")
			Ω(err).ShouldNot(HaveOccured())

			Eventually(func() interface{} {
				_, err = store.Get("/floop")
				return err
			}, 1.05, 0.01).Should(HaveOccured())
		})
	})
})
