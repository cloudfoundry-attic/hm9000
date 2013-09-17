package hm

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Walk", func() {
	var etcdStore store.Store
	BeforeEach(func() {
		conf, _ := config.DefaultConfig()
		etcdStore = store.NewETCDStore(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err := etcdStore.Connect()
		Ω(err).ShouldNot(HaveOccured())

		etcdStore.Set([]store.StoreNode{
			store.StoreNode{Key: "/desired-fresh", Value: []byte("123"), TTL: 0},
			store.StoreNode{Key: "/actual-fresh", Value: []byte("456"), TTL: 0},
			store.StoreNode{Key: "/desired/guid1", Value: []byte("guid1"), TTL: 0},
			store.StoreNode{Key: "/desired/guid2", Value: []byte("guid2"), TTL: 0},
			store.StoreNode{Key: "/menu/oj", Value: []byte("sweet"), TTL: 0},
			store.StoreNode{Key: "/menu/breakfast/pancakes", Value: []byte("tasty"), TTL: 0},
			store.StoreNode{Key: "/menu/breakfast/waffles", Value: []byte("delish"), TTL: 0},
		})
	})

	It("can recurse through keys in the store", func() {
		visited := make(map[string]string)
		walk(etcdStore, "/", func(node store.StoreNode) {
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
		walk(etcdStore, "/menu", func(node store.StoreNode) {
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
		walk(etcdStore, "/desired-fresh", func(node store.StoreNode) {
			called = true
		})

		Ω(called).Should(BeFalse())
	})
})
