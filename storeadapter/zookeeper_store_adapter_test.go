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
		creationTime time.Time
	)

	BeforeEach(func() {
		var err error
		client, _, err = zk.Connect(zookeeperRunner.NodeURLS(), time.Second)
		Ω(err).ShouldNot(HaveOccured())

		timeProvider = &faketimeprovider.FakeTimeProvider{}

		adapter = NewZookeeperStoreAdapter(zookeeperRunner.NodeURLS(), 100, timeProvider, time.Second)
		err = adapter.Connect()
		Ω(err).ShouldNot(HaveOccured())

		creationTime = time.Now()
		timeProvider.TimeToProvide = creationTime

		nodeArr = make([]StoreNode, 1)
	})

	AfterEach(func() {
		adapter.Disconnect()
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
						Ω(int(node.TTL)).Should(BeNumerically(">", 0))
						Ω(int(node.TTL)).Should(BeNumerically("<=", 2))
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

	Describe("List", func() {
		BeforeEach(func() {
			nodeArr = make([]StoreNode, 3)
			nodeArr[0] = StoreNode{
				Key:   "/restaurant/menu/breakfast",
				Value: []byte("waffle"),
				TTL:   10,
			}
			nodeArr[1] = StoreNode{
				Key:   "/restaurant/menu/lunch",
				Value: []byte("fried chicken"),
				TTL:   12,
			}
			nodeArr[2] = StoreNode{
				Key:   "/restaurant/menu/dinner/first_course",
				Value: []byte("snap peas"),
				TTL:   8,
			}

			err := adapter.Set(nodeArr)
			Ω(err).ShouldNot(HaveOccured())
		})

		Context("when the node exists, and is a directory", func() {
			It("should return the nodes in the directory", func() {
				nodes, err := adapter.List("/restaurant/menu")
				Ω(err).ShouldNot(HaveOccured())

				Ω(nodes).Should(HaveLen(3))
				Ω(nodes).Should(ContainElement(StoreNode{
					Key:   "/restaurant/menu/breakfast",
					Value: []byte("waffle"),
					TTL:   10,
					Dir:   false,
				}))
				Ω(nodes).Should(ContainElement(StoreNode{
					Key:   "/restaurant/menu/lunch",
					Value: []byte("fried chicken"),
					TTL:   12,
					Dir:   false,
				}))
				Ω(nodes).Should(ContainElement(StoreNode{
					Key:   "/restaurant/menu/dinner",
					Value: []byte{},
					TTL:   0,
					Dir:   true,
				}))
			})

			Context("when entries in the node have expired TTLs", func() {
				var nodes []StoreNode

				BeforeEach(func() {
					timeProvider.TimeToProvide = creationTime.Add(11 * time.Second)
					var err error
					nodes, err = adapter.List("/restaurant/menu")
					Ω(err).ShouldNot(HaveOccured())
				})

				It("does not return those entries in the result set", func() {
					Ω(nodes).Should(HaveLen(2))
					found := false
					for _, node := range nodes {
						if node.Key == "/restaurant/menu/lunch" {
							Ω(node.Value).Should(Equal([]byte("fried chicken")))
							Ω(node.TTL).Should(BeNumerically("<=", 2), "We've had some timing issues with this test.  Setting the expectation to <= 2 should help.")
							Ω(node.Dir).Should(BeFalse())
							found = true
						}
					}
					Ω(found).Should(BeTrue())
					Ω(nodes).Should(ContainElement(StoreNode{
						Key:   "/restaurant/menu/dinner",
						Value: []byte{},
						TTL:   0,
						Dir:   true,
					}))
				})

				It("deletes the expired entries", func() {
					_, _, err := client.Get("/restauraunt/menu/breakfast")
					Ω(err).Should(HaveOccured())
				})
			})

			Context("when the node is empty", func() {
				BeforeEach(func() {
					err := client.Delete("/restaurant/menu/dinner/first_course", -1)
					Ω(err).ShouldNot(HaveOccured())
				})

				It("should return an empty list without erroring", func() {
					nodes, err := adapter.List("/restaurant/menu/dinner")
					Ω(nodes).Should(HaveLen(0))
					Ω(err).ShouldNot(HaveOccured())
				})
			})
		})

		Context("when the node exists, but is not a directory", func() {
			It("should return an error", func() {
				nodes, err := adapter.List("/restaurant/menu/breakfast")
				Ω(nodes).Should(BeEmpty())
				Ω(err).Should(Equal(ErrorNodeIsNotDirectory))
			})
		})

		Context("when the node does not exist", func() {
			It("should return an error", func() {
				nodes, err := adapter.List("/not/a/real/node")
				Ω(nodes).Should(BeEmpty())
				Ω(err).Should(Equal(ErrorKeyNotFound))
			})
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			nodeArr[0] = StoreNode{
				Key:   "/restaurant/menu/breakfast",
				Value: []byte("waffle"),
				TTL:   10,
			}

			err := adapter.Set(nodeArr)
			Ω(err).ShouldNot(HaveOccured())
		})

		Context("when the key exists", func() {
			It("should delete the key", func() {
				err := adapter.Delete("/restaurant/menu/breakfast")
				Ω(err).ShouldNot(HaveOccured())
				_, err = adapter.Get("/restaurant/menu/breakfast")
				Ω(err).Should(Equal(ErrorKeyNotFound))
			})
		})

		Context("when the key is a directory", func() {
			It("should not delete the key and should return an is directory error", func() {
				err := adapter.Delete("/restaurant/menu")
				Ω(err).Should(Equal(ErrorNodeIsDirectory))
				_, err = adapter.Get("/restaurant/menu/breakfast")
				Ω(err).ShouldNot(HaveOccured())
			})
		})

		Context("when the key is an *empty* directory", func() {
			It("should delete the key", func() {
				err := adapter.Delete("/restaurant/menu/breakfast")
				Ω(err).ShouldNot(HaveOccured())
				err = adapter.Delete("/restaurant/menu")
				Ω(err).ShouldNot(HaveOccured())
				nodes, err := adapter.List("/restaurant")
				Ω(err).ShouldNot(HaveOccured())
				Ω(nodes).Should(BeEmpty())
			})
		})

		Context("when the key does not exist", func() {
			It("should return the missing key error", func() {
				err := adapter.Delete("/not/a/real/key")
				Ω(err).Should(Equal(ErrorKeyNotFound))
			})
		})
	})

	Describe("Empty nodes that aren't directories", func() {
		BeforeEach(func() {
			nodeArr[0] = StoreNode{Key: "/placeholder", Value: []byte{}}
			err := adapter.Set(nodeArr)
			Ω(err).ShouldNot(HaveOccured())
		})

		It("should allow the node to be retreived", func() {
			node, err := adapter.Get("/placeholder")
			Ω(node).Should(Equal(StoreNode{Key: "/placeholder", Value: []byte{}}))
			Ω(err).ShouldNot(HaveOccured())
		})

		It("should not allow listing the node", func() {
			nodes, err := adapter.List("/placeholder")
			Ω(nodes).Should(BeEmpty())
			Ω(err).Should(Equal(ErrorNodeIsNotDirectory))
		})
	})

})
