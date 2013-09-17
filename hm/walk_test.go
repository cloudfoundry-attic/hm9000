package hm

import (
	"github.com/cloudfoundry/hm9000/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Walk", func() {
	var etcdStore store.Store
	BeforeEach(func() {
		etcdStore = store.NewETCDStore(etcdRunner.NodeURLS())
		err := etcdStore.Connect()
		Ω(err).ShouldNot(HaveOccured())

		etcdStore.Set("/desired-fresh", []byte("123"), 1000)
		etcdStore.Set("/actual-fresh", []byte("456"), 1000)
		etcdStore.Set("/desired/guid1", []byte("guid1"), 1000)
		etcdStore.Set("/desired/guid2", []byte("guid2"), 1000)
		etcdStore.Set("/menu/oj", []byte("sweet"), 1000)
		etcdStore.Set("/menu/breakfast/pancakes", []byte("tasty"), 1000)
		etcdStore.Set("/menu/breakfast/waffles", []byte("delish"), 1000)
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
