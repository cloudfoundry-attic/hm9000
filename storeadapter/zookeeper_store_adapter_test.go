package storeadapter_test

import (
	. "github.com/cloudfoundry/hm9000/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/samuel/go-zookeeper/zk"
	"io/ioutil"
	"log"
	"os"

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

		Context("when setting a nested key", func() {
			BeforeEach(func() {
				nodeArr[0] = StoreNode{
					Key:   "/restaurant/menu/breakfast",
					Value: []byte("waffle"),
					TTL:   0,
				}
				err := adapter.Set(nodeArr)
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should be able to set the key", func() {
				data, stat, err := client.Get("/restaurant/menu/breakfast")
				Ω(string(data)).Should(Equal("waffle"))
				Ω(stat.NumChildren).Should(BeNumerically("==", 0))
				Ω(stat.Version).Should(BeNumerically("==", 0))
				Ω(err).ShouldNot(HaveOccured())

				acl, _, err := client.GetACL("/restaurant/menu/breakfast")
				Ω(acl).Should(Equal(zk.WorldACL(zk.PermAll)))
				Ω(err).ShouldNot(HaveOccured())

				_, stat, err = client.Get("/restaurant/menu")
				Ω(stat.NumChildren).Should(BeNumerically("==", 1))
				Ω(err).ShouldNot(HaveOccured())
			})

			Context("setting the key again", func() {
				BeforeEach(func() {
					nodeArr[0].Value = []byte("pancake")
					err := adapter.Set(nodeArr)
					Ω(err).Should(BeNil())
				})

				It("should be able to overwrite the key", func() {
					data, stat, err := client.Get("/restaurant/menu/breakfast")
					Ω(string(data)).Should(Equal("pancake"))
					Ω(stat.NumChildren).Should(BeNumerically("==", 0))
					Ω(stat.Version).Should(BeNumerically("==", 1))
					Ω(err).ShouldNot(HaveOccured())
				})
			})

			Context("setting a sibling key", func() {
				BeforeEach(func() {
					nodeArr[0] = StoreNode{
						Key:   "/restaurant/menu/lunch",
						Value: []byte("fried chicken"),
						TTL:   0,
					}
					err := adapter.Set(nodeArr)
					Ω(err).Should(BeNil())
				})

				It("should be able to overwrite the key", func() {
					kiddos, _, err := client.Children("/restaurant/menu")
					Ω(kiddos).Should(HaveLen(2))
					Ω(kiddos).Should(ContainElement("breakfast"))
					Ω(kiddos).Should(ContainElement("lunch"))
					Ω(err).ShouldNot(HaveOccured())
				})
			})
		})

		Context("when the store is down", func() {
			BeforeEach(func() {
				log.SetOutput(ioutil.Discard)
				zookeeperRunner.Stop()
			})

			AfterEach(func() {
				zookeeperRunner.Start()
				log.SetOutput(os.Stdout)
			})

			It("should return a timeout error", func() {
				nodeArr[0] = StoreNode{
					Key:   "/foo",
					Value: []byte("bar"),
					TTL:   0,
				}
				err := adapter.Set(nodeArr)
				Ω(IsTimeoutError(err)).Should(BeTrue(), "Expected a timeout error")
			})
		})
	})
})
