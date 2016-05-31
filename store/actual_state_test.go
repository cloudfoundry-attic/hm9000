package store_test

import (
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

var _ = FDescribe("Actual State", func() {
	var (
		store        Store
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

		results, err := store.GetInstanceHeartbeats()
		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(BeEmpty())

		results = store.GetCachedInstanceHeartbeats()
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
				results, err := store.GetInstanceHeartbeats()
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(0))

				err = store.EnsureCacheIsReady()
				Expect(err).NotTo(HaveOccurred())

				results = store.GetCachedInstanceHeartbeats()
				Expect(results).To(BeEmpty())
			})
		})

		Context("When the store contains heartbeats but the cache does not", func() {
			BeforeEach(func() {
				err := populate(dea.HeartbeatWith(
					dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
				))
				Expect(err).NotTo(HaveOccurred())

				results := store.GetCachedInstanceHeartbeats()
				Expect(results).To(HaveLen(0))

				results, err = store.GetInstanceHeartbeats()
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(2))
			})

			It("populates the local cache from the store", func() {
				err := store.EnsureCacheIsReady()
				Expect(err).NotTo(HaveOccurred())

				results := store.GetCachedInstanceHeartbeats()
				Expect(results).To(HaveLen(2))
				Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
				Expect(results).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))
			})
		})

		Context("when the store contains dea instances but the cache does not", func() {
			BeforeEach(func() {
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
				Expect(results[dea.DeaGuid].IsZero()).To(BeFalse())
			})

		})
	})

	Describe(".SyncHeartbeats", func() {
		JustBeforeEach(func() {
			store.SyncHeartbeats(dea.HeartbeatWith(
				dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
				dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
			))
		})

		Context("when saving heartbeats from a single DEA", func() {
			Context("when there are no heartbeats stored for the DEA in question", func() {
				BeforeEach(func() {
					err := store.EnsureCacheIsReady()
					Expect(err).NotTo(HaveOccurred())

					results := store.GetCachedInstanceHeartbeats()
					Expect(results).To(HaveLen(0))
				})

				It("stores the instance heartbeats in the cache", func() {
					results := store.GetCachedInstanceHeartbeats()
					Expect(results).To(HaveLen(2))
					Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Expect(results).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))
				})

				It("saves the instance heartbeats to the store", func() {
					results, err := store.GetInstanceHeartbeats()
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(2))
					Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Expect(results).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))
				})

				It("sets the dea instance on the cache with a ttl", func() { // cache is RealStore, store is etcd
					results := store.GetCachedDeaHeartbeats()
					Expect(results).To(HaveLen(1))
					Expect(results).To(HaveKey(dea.DeaGuid))
					Expect(results[dea.DeaGuid].IsZero()).To(BeFalse())
				})
			})

			Context("when there are already instance heartbeats stored for the DEA in question", func() {
				var (
					modifiedHeartbeat models.InstanceHeartbeat
					updateHeartbeat   = func() {
						modifiedHeartbeat = dea.GetApp(1).InstanceAtIndex(3).Heartbeat()
						modifiedHeartbeat.State = models.InstanceStateEvacuating
						store.SyncHeartbeats(dea.HeartbeatWith(
							modifiedHeartbeat,
							dea.GetApp(2).InstanceAtIndex(2).Heartbeat(),
						))
					}
				)

				It("syncs the heartbeats (add new ones, adjust ones that have changed state, and delete old ones)", func() {
					updateHeartbeat()
					results := store.GetCachedInstanceHeartbeats()
					// Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(2))
					Expect(results).To(ContainElement(modifiedHeartbeat))
					Expect(results).To(ContainElement(dea.GetApp(2).InstanceAtIndex(2).Heartbeat()))
					Expect(results).NotTo(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
				})

				It("Updates the expiration time for the cached dea", func() {
					results := store.GetCachedDeaHeartbeats()
					Expect(results).To(HaveLen(1))
					Expect(results).To(HaveKey(dea.DeaGuid))

					initialTime := results[dea.DeaGuid].UnixNano()

					updateHeartbeat()

					updatedResults := store.GetCachedDeaHeartbeats()
					Expect(updatedResults).To(HaveLen(1))
					Expect(updatedResults).To(HaveKey(dea.DeaGuid))

					Expect(initialTime).To(BeNumerically("<", updatedResults[dea.DeaGuid].UnixNano()))
				})
			})
		})

		Context("when saving heartbeats from multiple DEAs at once", func() {
			var modifiedHeartbeat models.InstanceHeartbeat
			var yetAnotherDea appfixture.DeaFixture

			JustBeforeEach(func() {
				yetAnotherDea = appfixture.NewDeaFixture()

				store.SyncHeartbeats(dea.HeartbeatWith(
					dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
				), otherDea.HeartbeatWith(
					otherDea.GetApp(3).InstanceAtIndex(0).Heartbeat(),
					otherDea.GetApp(2).InstanceAtIndex(1).Heartbeat(),
				), yetAnotherDea.HeartbeatWith(
					yetAnotherDea.GetApp(0).InstanceAtIndex(0).Heartbeat(),
				))
			})

			It("add all of the dea ID's to the dea cache", func() {
				results := store.GetCachedDeaHeartbeats()
				Expect(results).To(HaveLen(3))
				Expect(results).To(HaveKey(dea.DeaGuid))
				Expect(results).To(HaveKey(otherDea.DeaGuid))
				Expect(results).To(HaveKey(yetAnotherDea.DeaGuid))
			})

			Context("when we receive heartbeats from all DEAs at the same time", func() {
				It("saves all the heartbeats", func() {

					results, err := store.GetInstanceHeartbeats()
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(5))

					results = store.GetCachedInstanceHeartbeats()
					Expect(results).To(HaveLen(5))
				})
			})

			Context("when we receive heartbeats from a subset of the DEAs", func() {
				var (
					updateHeartbeat = func() {
						modifiedHeartbeat = dea.GetApp(1).InstanceAtIndex(3).Heartbeat()
						modifiedHeartbeat.State = models.InstanceStateEvacuating
						store.SyncHeartbeats(dea.HeartbeatWith(
							modifiedHeartbeat,
							dea.GetApp(2).InstanceAtIndex(2).Heartbeat(),
						), otherDea.HeartbeatWith(
							otherDea.GetApp(2).InstanceAtIndex(1).Heartbeat(),
							otherDea.GetApp(3).InstanceAtIndex(2).Heartbeat(),
						))
					}
				)

				It("Saves the heartbeats from those DEAs without deleting any from the DEAS from which it did not receive heartbeats", func() {
					updateHeartbeat()

					results, err := store.GetInstanceHeartbeats()
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(5))
					Expect(results).To(ContainElement(modifiedHeartbeat))
					Expect(results).To(ContainElement(dea.GetApp(2).InstanceAtIndex(2).Heartbeat()))
					Expect(results).To(ContainElement(otherDea.GetApp(2).InstanceAtIndex(1).Heartbeat()))
					Expect(results).To(ContainElement(otherDea.GetApp(3).InstanceAtIndex(2).Heartbeat()))
					Expect(results).To(ContainElement(yetAnotherDea.GetApp(0).InstanceAtIndex(0).Heartbeat()))

					Expect(results).NotTo(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Expect(results).NotTo(ContainElement(otherDea.GetApp(3).InstanceAtIndex(0).Heartbeat()))
				})

				// maybe remove?
				It("Updates the expiration time for the received cached DEAs", func() {
					results := store.GetCachedDeaHeartbeats()
					Expect(results).To(HaveLen(3))
					Expect(results).To(HaveKey(dea.DeaGuid))
					Expect(results).To(HaveKey(otherDea.DeaGuid))
					Expect(results).To(HaveKey(yetAnotherDea.DeaGuid))

					deaTime := results[dea.DeaGuid].UnixNano()
					otherDeaTime := results[otherDea.DeaGuid].UnixNano()
					yetAnotherDeaTime := results[yetAnotherDea.DeaGuid].UnixNano()

					updateHeartbeat()

					updatedResults := store.GetCachedDeaHeartbeats()
					Expect(results).To(HaveLen(3))
					Expect(results).To(HaveKey(dea.DeaGuid))
					Expect(results).To(HaveKey(otherDea.DeaGuid))
					Expect(results).To(HaveKey(yetAnotherDea.DeaGuid))

					Expect(deaTime).To(BeNumerically("<", updatedResults[dea.DeaGuid].UnixNano()))
					Expect(otherDeaTime).To(BeNumerically("<", updatedResults[otherDea.DeaGuid].UnixNano()))
					Expect(yetAnotherDeaTime).To(Equal(updatedResults[yetAnotherDea.DeaGuid].UnixNano()))
				})
			})
		})

		Context("when the in-memory cache no longer matches the store", func() {
			JustBeforeEach(func() {
				results, err := store.GetInstanceHeartbeats()
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(2))

				results = store.GetCachedInstanceHeartbeats()
				Expect(results).To(HaveLen(2))
			})

			It("adds any incoming dea heartbeats to the store", func() {
				err := storeAdapter.Delete("/hm/v1/dea-presence/" + dea.DeaGuid)
				Expect(err).ToNot(HaveOccurred())

				err = store.SyncHeartbeats(dea.HeartbeatWith(
					dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
				))
				Expect(err).ToNot(HaveOccurred())

				results := store.GetCachedDeaHeartbeats()
				Expect(results).To(HaveLen(1))
				Expect(results).To(HaveKey(dea.DeaGuid))

				nodes, err := store.GetDeaHeartbeats()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(nodes)).To(Equal(1))
				Expect(nodes[0].Key).To(ContainSubstring(dea.DeaGuid))
			})

			It("recovers by updating the store when it receives the next heartbeat", func() {
				corruptedHeartbeat := dea.GetApp(0).InstanceAtIndex(1).Heartbeat()
				storeAdapter.Delete("/hm/v1/apps/actual/" + store.AppKey(corruptedHeartbeat.AppGuid, corruptedHeartbeat.AppVersion) + "/" + corruptedHeartbeat.InstanceGuid)

				results, err := store.GetInstanceHeartbeats()
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(1))

				results = store.GetCachedInstanceHeartbeats()
				Expect(results).To(HaveLen(2))

				store.SyncHeartbeats(dea.HeartbeatWith(
					dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
				))

				results, err = store.GetInstanceHeartbeats()
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(2))
			})
		})
	})

	Describe("SyncCacheToStore", func() {
		JustBeforeEach(func() {
			store.SyncHeartbeats(dea.HeartbeatWith(
				dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
				dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
			))
		})

		It("syncs the dea cache to the store", func() {
			err := storeAdapter.Delete("/hm/v1/dea-presence/" + dea.DeaGuid)
			Expect(err).ToNot(HaveOccurred())

			results := store.GetCachedDeaHeartbeats()
			Expect(results).To(HaveLen(1))

			nodes, err := store.GetDeaHeartbeats()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodes)).To(Equal(0))

			err = store.SyncCacheToStore()
			Expect(err).NotTo(HaveOccurred())

			nodes, err = store.GetDeaHeartbeats()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(nodes)).To(Equal(1))
			Expect(nodes[0].Key).To(ContainSubstring(dea.DeaGuid))
		})

		It("syncs the cache to the store", func() {
			err := populate(dea.HeartbeatWith(
				dea.GetApp(2).InstanceAtIndex(1).Heartbeat(),
				dea.GetApp(4).InstanceAtIndex(0).Heartbeat(),
			))
			Expect(err).NotTo(HaveOccurred())

			corruptedHeartbeat := dea.GetApp(0).InstanceAtIndex(1).Heartbeat()
			storeAdapter.Delete("/hm/v1/apps/actual/" + store.AppKey(corruptedHeartbeat.AppGuid, corruptedHeartbeat.AppVersion) + "/" + corruptedHeartbeat.InstanceGuid)

			results := store.GetCachedInstanceHeartbeats()
			Expect(results).To(HaveLen(2))
			Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
			Expect(results).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))

			results, err = store.GetInstanceHeartbeats()
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(3))

			err = store.SyncCacheToStore()
			Expect(err).NotTo(HaveOccurred())

			results, err = store.GetInstanceHeartbeats()
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(2))
			Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
			Expect(results).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))

			results = store.GetCachedInstanceHeartbeats()
			Expect(results).To(HaveLen(2))
			Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
			Expect(results).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))
		})
	})

	Describe("Fetching all actual state", func() {
		Context("when there is none saved", func() {
			It("comes back empty", func() {
				err := store.EnsureCacheIsReady()
				Expect(err).NotTo(HaveOccurred())

				results := store.GetCachedInstanceHeartbeats()
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
					results, err := store.GetInstanceHeartbeats()
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
					results, err := store.GetInstanceHeartbeats()
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(3))
					Expect(results).To(ContainElement(otherDea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Expect(results).To(ContainElement(otherDea.GetApp(1).InstanceAtIndex(0).Heartbeat()))
					Expect(results).To(ContainElement(heartbeatOnOtherDea))

					//we fetch twice to ensure that nothing is incorrectly deleted
					results, err = store.GetInstanceHeartbeats()
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

					_, err = store.GetInstanceHeartbeats()
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
							results, err := store.GetInstanceHeartbeats()
							resultChan <- results
							errChan <- err
						}()

						go func() {
							results, err := store.GetInstanceHeartbeats()
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
				results, err := store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
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
					results, err := store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
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
					results, err := store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(1))
					Expect(results).To(ContainElement(heartbeatB))
				})

				It("removes expired instance heartbeats from the store", func() {
					_, err := storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(app.AppGuid, app.AppVersion) + "/" + heartbeatA.StoreKey())
					Expect(err).NotTo(HaveOccurred())

					_, err = store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Expect(err).NotTo(HaveOccurred())

					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(app.AppGuid, app.AppVersion) + "/" + heartbeatA.StoreKey())
					Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
				})

				Context("if it fails to remove them", func() {
					It("should soldier on", func() {
						resultChan := make(chan []models.InstanceHeartbeat, 2)
						errChan := make(chan error, 2)
						go func() {
							results, err := store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
							resultChan <- results
							errChan <- err
						}()

						go func() {
							results, err := store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
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
					results, err := store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(results).NotTo(BeNil())
					Expect(results).To(HaveLen(0))
				})
			})
		})
	})
})
