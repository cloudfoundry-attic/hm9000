package store_test

import (
	"time"

	"github.com/cloudfoundry/gunk/workpool"
	. "github.com/cloudfoundry/hm9000/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
)

var _ = Describe("Actual State", func() {
	var (
		store        *RealStore
		storeAdapter storeadapter.StoreAdapter
		conf         *config.Config
		dea          appfixture.DeaFixture
		otherDea     appfixture.DeaFixture
		populate     func(...*models.Heartbeat) error
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
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
		conf.StoreHeartbeatCacheRefreshIntervalInMilliseconds = 100
		store = NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())

		dea = appfixture.NewDeaFixture()
		otherDea = appfixture.NewDeaFixture()

		results, err := store.GetStoredInstanceHeartbeats()
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(BeEmpty())

		results, err = store.GetCachedInstanceHeartbeats()
		Expect(err).ToNot(HaveOccurred())
		Expect(results).To(BeEmpty())

		populate = func(heartbeats ...*models.Heartbeat) error {
			nodes := make([]storeadapter.StoreNode, 0)

			for _, hb := range heartbeats {
				nodes = append(nodes, storeadapter.StoreNode{
					Key:   "/hm/v1/dea-presence/" + hb.DeaGuid,
					Value: []byte(hb.DeaGuid),
					TTL:   conf.HeartbeatTTL(),
				})

				for _, ihb := range hb.InstanceHeartbeats {
					nodes = append(nodes, storeadapter.StoreNode{
						Key:   "/hm/v1/apps/actual/" + store.AppKey(ihb.AppGuid, ihb.AppVersion) + "/" + ihb.InstanceGuid,
						Value: ihb.ToCSV(),
					})
				}
			}

			return storeAdapter.SetMulti(nodes)
		}
	})

	AfterEach(func() {
		storeAdapter.Disconnect()
	})

	Describe(".EnsureCacheIsReady", func() {
		Context("When the store is empty", func() {
			It("creates an empty cache", func() {
				results, err := store.GetStoredInstanceHeartbeats()
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(0))

				err = store.EnsureCacheIsReady()
				Expect(err).NotTo(HaveOccurred())

				results, err = store.GetCachedInstanceHeartbeats()
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(BeEmpty())
			})
		})

		Context("When the store contains heartbeats but the cache does not", func() {
			JustBeforeEach(func() {
				err := populate(dea.HeartbeatWith(
					dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
				))
				Expect(err).NotTo(HaveOccurred())

				store.AddDeaHeartbeats([]string{dea.DeaGuid})

				results, err := store.GetCachedInstanceHeartbeats()
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(0))

				results, err = store.GetStoredInstanceHeartbeats()
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(2))
			})

			It("populates the local cache from the store", func() {
				err := store.EnsureCacheIsReady()
				Expect(err).NotTo(HaveOccurred())

				results, err := store.GetCachedInstanceHeartbeats()
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(HaveLen(2))
				Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
				Expect(results).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))

				Expect(len(store.InstanceHeartbeatCache())).To(Equal(2))
				Expect(store.InstanceHeartbeatCache()[dea.GetApp(0).AppGuid+","+dea.GetApp(0).AppVersion]).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
				Expect(store.InstanceHeartbeatCache()[dea.GetApp(1).AppGuid+","+dea.GetApp(1).AppVersion]).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))
			})
		})

		Context("when the store contains dea instances but the cache does not", func() {
			JustBeforeEach(func() {
				err := populate(dea.HeartbeatWith(
					dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
				))
				Expect(err).NotTo(HaveOccurred())

				results := store.GetCachedDeaHeartbeats()
				Expect(results).To(HaveLen(0))

				summaryNodes, err := storeAdapter.ListRecursively("/hm/v1/dea-presence/")
				Expect(err).ToNot(HaveOccurred())
				Expect(len(summaryNodes.ChildNodes)).To(Equal(1))
				Expect(summaryNodes.ChildNodes[0].Key).To(Equal("/hm/v1/dea-presence/" + dea.DeaGuid))
			})

			It("populates the local cache from the store", func() {
				err := store.EnsureCacheIsReady()
				Expect(err).NotTo(HaveOccurred())

				results := store.GetCachedDeaHeartbeats()
				Expect(results).To(HaveLen(1))
				Expect(results).To(HaveKey(dea.DeaGuid))
				Expect(results[dea.DeaGuid]).To(BeNumerically(">", time.Now().UnixNano()))
			})
		})
	})

	Describe("Fetching all actual state", func() {
		Context("when there is none saved", func() {
			It("comes back empty", func() {
				err := store.EnsureCacheIsReady()
				Expect(err).NotTo(HaveOccurred())

				results, err := store.GetCachedInstanceHeartbeats()
				Expect(err).ToNot(HaveOccurred())
				Expect(results).To(BeEmpty())
			})
		})

		Context("when there is actual state saved", func() {
			var heartbeatOnDea, heartbeatOnOtherDea models.InstanceHeartbeat

			BeforeEach(func() {
				appOnBothDeas := appfixture.NewAppFixture()

				heartbeatOnDea = appOnBothDeas.InstanceAtIndex(0).Heartbeat()
				heartbeatOnDea.DeaGuid = dea.DeaGuid

				heartbeatOnOtherDea = appOnBothDeas.InstanceAtIndex(1).Heartbeat()
				heartbeatOnOtherDea.DeaGuid = otherDea.DeaGuid

				store.SyncHeartbeats(dea.HeartbeatWith(
					dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
					heartbeatOnDea,
				))

				store.SyncHeartbeats(otherDea.HeartbeatWith(
					otherDea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					otherDea.GetApp(1).InstanceAtIndex(0).Heartbeat(),
					heartbeatOnOtherDea,
				))
			})

			Context("when the DEA heartbeats have not expired", func() {
				It("returns the instance heartbeats", func() {
					results, err := store.GetStoredInstanceHeartbeats()
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(6))
					Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Expect(results).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))
					Expect(results).To(ContainElement(heartbeatOnDea))
					Expect(results).To(ContainElement(otherDea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Expect(results).To(ContainElement(otherDea.GetApp(1).InstanceAtIndex(0).Heartbeat()))
					Expect(results).To(ContainElement(heartbeatOnOtherDea))
				})
			})

			Context("when a DEA heartbeat has expired", func() {
				BeforeEach(func() {
					storeAdapter.Delete("/hm/v1/dea-presence/" + dea.DeaGuid)
				})

				It("does not return any expired instance heartbeats", func() {
					results, err := store.GetStoredInstanceHeartbeats()
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(3))
					Expect(results).To(ContainElement(otherDea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Expect(results).To(ContainElement(otherDea.GetApp(1).InstanceAtIndex(0).Heartbeat()))
					Expect(results).To(ContainElement(heartbeatOnOtherDea))

					//we fetch twice to ensure that nothing is incorrectly deleted
					results, err = store.GetStoredInstanceHeartbeats()
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(3))
					Expect(results).To(ContainElement(otherDea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Expect(results).To(ContainElement(otherDea.GetApp(1).InstanceAtIndex(0).Heartbeat()))
					Expect(results).To(ContainElement(heartbeatOnOtherDea))
				})

				It("removes expired instance heartbeats from the store", func() {
					_, err := storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(dea.GetApp(0).AppGuid, dea.GetApp(0).AppVersion) + "/" + dea.GetApp(0).InstanceAtIndex(1).Heartbeat().StoreKey())
					Expect(err).NotTo(HaveOccurred())
					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(dea.GetApp(1).AppGuid, dea.GetApp(1).AppVersion) + "/" + dea.GetApp(1).InstanceAtIndex(3).Heartbeat().StoreKey())
					Expect(err).NotTo(HaveOccurred())

					_, err = store.GetStoredInstanceHeartbeats()
					Expect(err).NotTo(HaveOccurred())

					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(dea.GetApp(0).AppGuid, dea.GetApp(0).AppVersion) + "/" + dea.GetApp(0).InstanceAtIndex(1).Heartbeat().StoreKey())
					Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(dea.GetApp(1).AppGuid, dea.GetApp(1).AppVersion) + "/" + dea.GetApp(1).InstanceAtIndex(3).Heartbeat().StoreKey())
					Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
				})

				Context("if it fails to remove them", func() {
					It("should soldier on", func() {
						resultChan := make(chan []models.InstanceHeartbeat, 2)
						errChan := make(chan error, 2)
						go func() {
							results, err := store.GetStoredInstanceHeartbeats()
							resultChan <- results
							errChan <- err
						}()

						go func() {
							results, err := store.GetStoredInstanceHeartbeats()
							resultChan <- results
							errChan <- err
						}()

						Expect(<-resultChan).To(HaveLen(3))
						Expect(<-resultChan).To(HaveLen(3))
						Expect(<-errChan).NotTo(HaveOccurred())
						Expect(<-errChan).NotTo(HaveOccurred())
					})
				})
			})
		})
	})

	Describe("Fetching actual state for a specific app guid & version", func() {
		var app appfixture.AppFixture
		BeforeEach(func() {
			app = appfixture.NewAppFixture()
		})

		Context("when there is none saved", func() {
			It("comes back empty", func() {
				results, err := store.GetCachedInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(BeEmpty())
			})
		})

		Context("when there is actual state saved", func() {
			var heartbeatA, heartbeatB models.InstanceHeartbeat

			BeforeEach(func() {
				heartbeatA = app.InstanceAtIndex(0).Heartbeat()
				heartbeatA.DeaGuid = "A"

				store.SyncHeartbeats(&models.Heartbeat{
					DeaGuid: "A",
					InstanceHeartbeats: []models.InstanceHeartbeat{
						heartbeatA,
					},
				})

				heartbeatB = app.InstanceAtIndex(1).Heartbeat()
				heartbeatB.DeaGuid = "B"

				store.SyncHeartbeats(&models.Heartbeat{
					DeaGuid: "B",
					InstanceHeartbeats: []models.InstanceHeartbeat{
						heartbeatB,
					},
				})
			})

			Context("when the corresponding DEA heartbeat has not expired", func() {
				It("returns the instance heartbeats", func() {
					results, err := store.GetCachedInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(2))
					Expect(results).To(ContainElement(heartbeatA))
					Expect(results).To(ContainElement(heartbeatB))
				})
			})

			Context("when an associated DEA heartbeat has expired", func() {
				BeforeEach(func() {
					storeAdapter.Delete("/hm/v1/dea-presence/A")
				})

				It("does not return any expired instance heartbeats", func() {
					results, err := store.GetStoredInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(1))
					Expect(results).To(ContainElement(heartbeatB))
				})

				FIt("removes expired instance heartbeats from the store", func() {
					_, err := storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(app.AppGuid, app.AppVersion) + "/" + heartbeatA.StoreKey())
					Expect(err).NotTo(HaveOccurred())

					_, err = store.GetCachedInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Expect(err).NotTo(HaveOccurred())

					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(app.AppGuid, app.AppVersion) + "/" + heartbeatA.StoreKey())
					Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
				})

				Context("if it fails to remove them", func() {
					It("should soldier on", func() {
						resultChan := make(chan []models.InstanceHeartbeat, 2)
						errChan := make(chan error, 2)
						go func() {
							results, err := store.GetCachedInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
							resultChan <- results
							errChan <- err
						}()

						go func() {
							results, err := store.GetCachedInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
							resultChan <- results
							errChan <- err
						}()

						Expect(<-resultChan).To(HaveLen(1))
						Expect(<-resultChan).To(HaveLen(1))
						Expect(<-errChan).NotTo(HaveOccurred())
						Expect(<-errChan).NotTo(HaveOccurred())
					})
				})
			})

			Context("when all the DEA heartbeats have expired", func() {
				BeforeEach(func() {
					storeAdapter.Delete("/hm/v1/dea-presence/A", "/hm/v1/dea-presence/B")
				})

				It("does not return any instance heartbeats", func() {
					results, err := store.GetCachedInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(results).NotTo(BeNil())
					Expect(results).To(HaveLen(0))
				})
			})
		})
	})
})
