package hm_test

import (
	"github.com/cloudfoundry/hm9000/config"
	. "github.com/cloudfoundry/hm9000/hm"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Store", func() {
	var (
		etcdStoreAdapter storeadapter.StoreAdapter
		nodes            []storeadapter.StoreNode
		conf             config.Config
	)

	BeforeEach(func() {
		conf, _ = config.DefaultConfig()
		etcdStoreAdapter = storeadapter.NewETCDStoreAdapter(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err := etcdStoreAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())

		nodes = []storeadapter.StoreNode{
			storeadapter.StoreNode{Key: "/desired-fresh", Value: []byte("123"), TTL: 0},
			storeadapter.StoreNode{Key: "/actual-fresh", Value: []byte("456"), TTL: 0},
			storeadapter.StoreNode{Key: "/desired/guid1", Value: []byte("guid1"), TTL: 0},
			storeadapter.StoreNode{Key: "/desired/guid2", Value: []byte("guid2"), TTL: 0},
			storeadapter.StoreNode{Key: "/menu/oj", Value: []byte("sweet"), TTL: 0},
			storeadapter.StoreNode{Key: "/menu/breakfast/pancakes", Value: []byte("tasty"), TTL: 0},
			storeadapter.StoreNode{Key: "/menu/breakfast/waffles", Value: []byte("delish"), TTL: 0},
		}
		etcdStoreAdapter.Set(nodes)
	})

	Describe("Walk", func() {
		It("can recurse through keys in the store", func() {
			visited := make(map[string]string)
			Walk(etcdStoreAdapter, "/", func(node storeadapter.StoreNode) {
				visited[node.Key] = string(node.Value)
			})

			Ω(visited).Should(Equal(map[string]string{
				"/desired-fresh":           "123",
				"/actual-fresh":            "456",
				"/desired/guid1":           "guid1",
				"/desired/guid2":           "guid2",
				"/menu/oj":                 "sweet",
				"/menu/breakfast/pancakes": "tasty",
				"/menu/breakfast/waffles":  "delish",
			}))
		})

		It("can recurse through keys in the store at an arbitrary level", func() {
			visited := make(map[string]string)
			Walk(etcdStoreAdapter, "/menu", func(node storeadapter.StoreNode) {
				visited[node.Key] = string(node.Value)
			})

			Ω(visited).Should(Equal(map[string]string{
				"/menu/oj":                 "sweet",
				"/menu/breakfast/pancakes": "tasty",
				"/menu/breakfast/waffles":  "delish",
			}))
		})

		It("doesn't call the callback when passed a non-directory", func() {
			called := false
			Walk(etcdStoreAdapter, "/desired-fresh", func(node storeadapter.StoreNode) {
				called = true
			})

			Ω(called).Should(BeFalse())
		})
	})

	Describe("Clear", func() {
		It("deletes all entries from store", func() {
			conf.StoreURLs = etcdRunner.NodeURLS()
			Clear(fakelogger.NewFakeLogger(), conf)
			for _, node := range nodes {
				_, err := etcdStoreAdapter.Get(node.Key)
				Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))
			}
		})
	})
})
