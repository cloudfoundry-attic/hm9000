package storeadapter_test

import (
	. "github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"
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
		adapter      StoreAdapter
		client       *zk.Conn
		nodeArr      []StoreNode
		timeProvider *faketimeprovider.FakeTimeProvider
	)

	BeforeEach(func() {
		var err error
		client, _, err = zk.Connect(zookeeperRunner.NodeURLS(), time.Second)
		Ω(err).ShouldNot(HaveOccured())

		timeProvider = &faketimeprovider.FakeTimeProvider{}

		adapter = NewZookeeperStoreAdapter(zookeeperRunner.NodeURLS(), 100, timeProvider, time.Second)
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
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should be able to set the key", func() {
				data, stat, err := client.Get("/foo")
				Ω(string(data)).Should(Equal("0,bar"))
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
					nodeArr[0].TTL = 20
					err := adapter.Set(nodeArr)
					Ω(err).ShouldNot(HaveOccured())
				})

				It("should be able to overwrite the key", func() {
					data, stat, err := client.Get("/foo")
					Ω(string(data)).Should(Equal("20,baz"))
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
				Ω(string(data)).Should(Equal("0,waffle"))
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
					Ω(err).ShouldNot(HaveOccured())
				})

				It("should be able to overwrite the key", func() {
					data, stat, err := client.Get("/restaurant/menu/breakfast")
					Ω(string(data)).Should(Equal("0,pancake"))
					Ω(stat.NumChildren).Should(BeNumerically("==", 0))
					Ω(stat.Version).Should(BeNumerically("==", 1))
					Ω(err).ShouldNot(HaveOccured())
				})
			})

			Context("setting a directory", func() {
				It("should return a StoreErrorIsDirectory", func() {
					nodeArr[0] = StoreNode{
						Key:   "/restaurant/menu",
						Value: []byte("french toast"),
						TTL:   0,
					}
					err := adapter.Set(nodeArr)
					Ω(err).Should(Equal(ErrorNodeIsDirectory), "Expecting a StoreErrorIsDirectory")
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
					Ω(err).ShouldNot(HaveOccured())
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
				Ω(err).Should(Equal(ErrorTimeout), "Expected a timeout error")
			})
		})
	})

	Describe("Get", func() {
		var creationTime time.Time

		BeforeEach(func() {
			creationTime = time.Now()
			timeProvider.TimeToProvide = creationTime
		})

		Context("when the node exists", func() {
			BeforeEach(func() {
				nodeArr[0] = StoreNode{
					Key:   "/restaurant/menu/breakfast",
					Value: []byte("waffle,banana"),
					TTL:   30,
				}
				err := adapter.Set(nodeArr)
				Ω(err).ShouldNot(HaveOccured())
			})

			Context("and the node has no children and is still alive", func() {
				It("returns the contents of the node", func() {
					node, err := adapter.Get("/restaurant/menu/breakfast")
					Ω(err).ShouldNot(HaveOccured())
					Ω(node.Key).Should(Equal("/restaurant/menu/breakfast"))
					Ω(string(node.Value)).Should(Equal("waffle,banana"))
					Ω(int(node.TTL)).Should(Equal(30))
					Ω(node.Dir).Should(BeFalse())
				})
			})

			Context("and the node has children", func() {
				It("returns the StoreErrorIsDirectory error", func() {
					node, err := adapter.Get("/restaurant/menu")
					Ω(err).Should(Equal(ErrorNodeIsDirectory))
					Ω(node).Should(BeZero())
				})
			})

			Context("when time elapses", func() {
				Context("and the node's TTL has not expired", func() {
					BeforeEach(func() {
						timeProvider.TimeToProvide = creationTime.Add(29 * time.Second)
					})

					It("returns the node with the correct TTL", func() {
						node, err := adapter.Get("/restaurant/menu/breakfast")
						Ω(err).ShouldNot(HaveOccured())
						Ω(int(node.TTL)).Should(Equal(1))
					})
				})

				Context("when time went backwards because clocks aren't 100% in sync", func() {
					BeforeEach(func() {
						timeProvider.TimeToProvide = creationTime.Add(-10 * time.Second)
					})

					It("returns the node without modifying the TTL", func() {
						node, err := adapter.Get("/restaurant/menu/breakfast")
						Ω(err).ShouldNot(HaveOccured())
						Ω(int(node.TTL)).Should(Equal(30))
					})
				})

				Context("and the node's TTL has expired", func() {
					BeforeEach(func() {
						_, _, err := client.Get("/restaurant/menu/breakfast")
						Ω(err).ShouldNot(HaveOccured())

						timeProvider.TimeToProvide = creationTime.Add(31 * time.Second)
					})

					It("returns the StoreErrorKeyNotFound error", func() {
						node, err := adapter.Get("/restaurant/menu/breakfast")
						Ω(err).Should(Equal(ErrorKeyNotFound))
						Ω(node).Should(BeZero())
					})

					It("deletes the node", func() {
						adapter.Get("/restaurant/menu/breakfast")
						_, _, err := client.Get("/restaurant/menu/breakfast")
						Ω(err).Should(HaveOccured())
					})
				})
			})
		})

		Context("when the node has a TTL of 0", func() {
			BeforeEach(func() {
				nodeArr[0] = StoreNode{
					Key:   "/restaurant/menu/breakfast",
					Value: []byte("waffle"),
					TTL:   0,
				}
				err := adapter.Set(nodeArr)
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should never mark the node as expired", func() {
				timeProvider.TimeToProvide = creationTime.Add(24 * time.Hour)
				node, err := adapter.Get("/restaurant/menu/breakfast")
				Ω(err).ShouldNot(HaveOccured())
				Ω(string(node.Value)).Should(Equal("waffle"))
				Ω(int(node.TTL)).Should(Equal(0))
			})
		})

		Context("when the node does not exist", func() {
			It("returns the StoreErrorKeyNotFound error", func() {
				node, err := adapter.Get("/no/node/for/you")
				Ω(err).Should(Equal(ErrorKeyNotFound))
				Ω(node).Should(BeZero())
			})
		})

		Context("when the node has an invalid format", func() {
			BeforeEach(func() {
				client.Create("/missingTTL", []byte("waffle"), 0, zk.WorldACL(zk.PermAll))
				client.Create("/invalidTTL", []byte("a,waffle"), 0, zk.WorldACL(zk.PermAll))
			})

			It("returns the StoreErrorInvalidFormat error", func() {
				node, err := adapter.Get("/missingTTL")
				Ω(err).Should(Equal(ErrorInvalidFormat), "Expected the error to be an IsInvalidFormatError error")
				Ω(node).Should(BeZero())

				node, err = adapter.Get("/invalidTTL")
				Ω(err).Should(Equal(ErrorInvalidFormat), "Expected the error to be an IsInvalidFormatError error")
				Ω(node).Should(BeZero())
			})
		})
	})

	Describe("List", func() {})
	Describe("Delete", func() {})
})
