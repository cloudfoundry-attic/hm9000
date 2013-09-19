package storeadapter_test

import (
	. "github.com/cloudfoundry/hm9000/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/samuel/go-zookeeper/zk"

	"time"
)

var _ = Describe("ZookeeperStoreAdapter", func() {
	var (
		adapter StoreAdapter
		client  *zk.Conn
		nodeArr []StoreNode
	)

	BeforeEach(func() {
		var err error
		client, _, err = zk.Connect(zookeeperRunner.NodeURLS(), time.Second)
		Ω(err).ShouldNot(HaveOccured())

		adapter = NewZookeeperStoreAdapter(zookeeperRunner.NodeURLS(), 100, time.Second)
		err = adapter.Connect()
		Ω(err).ShouldNot(HaveOccured())

		nodeArr = make([]StoreNode, 1)
	})

	Describe("Set", func() {
		Context("when setting a shallow key", func() {
			BeforeEach(func() {
				nodeArr[0] = StoreNode{
					Key:   "/foo",
					Value: []byte("bar"),
					TTL:   0,
				}
				err := adapter.Set(nodeArr)
				Ω(err).Should(BeNil())
			})

			It("should be able to set the key", func() {
				data, stat, err := client.Get("/foo")
				Ω(string(data)).Should(Equal("bar"))
				Ω(stat.NumChildren).Should(BeNumerically("==", 0))
				Ω(stat.Version).Should(BeNumerically("==", 0))
				Ω(err).ShouldNot(HaveOccured())

				acl, _, err := client.GetACL("/foo")
				Ω(acl).Should(Equal(zk.WorldACL(zk.PermAll)))
				Ω(err).ShouldNot(HaveOccured())
			})

			Context("setting the key again", func() {
				BeforeEach(func() {
					nodeArr[0].Value = []byte("baz")
					err := adapter.Set(nodeArr)
					Ω(err).Should(BeNil())
				})

				It("should be able to overwrite the key", func() {
					data, stat, err := client.Get("/foo")
					Ω(string(data)).Should(Equal("baz"))
					Ω(stat.NumChildren).Should(BeNumerically("==", 0))
					Ω(stat.Version).Should(BeNumerically("==", 1))
					Ω(err).ShouldNot(HaveOccured())
				})
			})
		})

		// Context("when setting a nested key", func() {
		// 	BeforeEach(func() {
		// 		nodeArr[0] = StoreNode{
		// 			Key:   "/menu/breakfast",
		// 			Value: []byte("waffle"),
		// 			TTL:   0,
		// 		}
		// 		err := adapter.Set(nodeArr)
		// 		Ω(err).Should(BeNil())
		// 	})

		// 	It("should be able to set the key", func() {
		// 		data, stat, err := client.Get("/menu/waffle")
		// 		Ω(string(data)).Should(Equal("breakfast"))
		// 		Ω(stat.NumChildren).Should(BeNumerically("==", 0))
		// 		Ω(stat.Version).Should(BeNumerically("==", 0))
		// 		Ω(err).ShouldNot(HaveOccured())

		// 		acl, _, err := client.GetACL("/menu/waffle")
		// 		Ω(acl).Should(Equal(zk.WorldACL(zk.PermAll)))
		// 		Ω(err).ShouldNot(HaveOccured())

		// 		_, stat, err = client.Get("/menu")
		// 		Ω(stat.NumChildren).Should(BeNumerically("==", 2))
		// 		Ω(err).ShouldNot(HaveOccured())
		// 	})
		// })
	})
})
