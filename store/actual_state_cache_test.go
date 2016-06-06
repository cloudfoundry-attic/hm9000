package store_test

import (
	"errors"
	"time"

	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/fakes"

	. "github.com/cloudfoundry/hm9000/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cached actual state", func() {
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

					results, err := store.GetCachedInstanceHeartbeats()
					Expect(err).ToNot(HaveOccurred())
					Expect(results).To(HaveLen(0))
				})

				It("stores the instance heartbeats in the cache", func() {
					results, err := store.GetCachedInstanceHeartbeats()
					Expect(err).ToNot(HaveOccurred())
					Expect(results).To(HaveLen(2))
					Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Expect(results).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))
				})

				It("saves the instance heartbeats to the store", func() {
					results, err := store.GetStoredInstanceHeartbeats()
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(2))
					Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Expect(results).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))
				})

				It("sets the dea instance on the cache with a ttl", func() {
					results := store.GetCachedDeaHeartbeats()
					Expect(results).To(HaveLen(1))
					Expect(results).To(HaveKey(dea.DeaGuid))
					Expect(results[dea.DeaGuid]).To(BeNumerically(">", 0))
				})

				It("saves the dea to the store", func() {
					result, err := store.GetStoredDeaHeartbeats()
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(HaveLen(1))
					Expect(result[0].Key).To(ContainSubstring(dea.DeaGuid))
				})
			})

			Context("when there are already instance heartbeats stored for the DEA in question", func() {
				It("adds the new instance heartbeats to the cache and store on the next dea heartbeat", func() {
					store.SyncHeartbeats(dea.HeartbeatWith(
						dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
						dea.GetApp(2).InstanceAtIndex(2).Heartbeat(),
					))

					app1 := dea.GetApp(1)
					app2 := dea.GetApp(2)

					cachedResults := store.InstanceHeartbeatCache()
					Expect(cachedResults).To(HaveLen(2))
					Expect(cachedResults[store.AppKey(app1.AppGuid, app1.AppVersion)]).To(ContainElement(app1.InstanceAtIndex(3).Heartbeat()))
					Expect(cachedResults[store.AppKey(app2.AppGuid, app2.AppVersion)]).To(ContainElement(app2.InstanceAtIndex(2).Heartbeat()))

					results, err := store.GetStoredInstanceHeartbeats()
					Expect(err).ToNot(HaveOccurred())
					Expect(results).To(HaveLen(2))
					Expect(results).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))
					Expect(results).To(ContainElement(dea.GetApp(2).InstanceAtIndex(2).Heartbeat()))
				})

				It("updates the instance heartbeats that have changed state on the next dea heartbeat", func() {
					modifiedHeartbeat := dea.GetApp(1).InstanceAtIndex(3).Heartbeat()
					modifiedHeartbeat.State = models.InstanceStateEvacuating
					store.SyncHeartbeats(dea.HeartbeatWith(
						modifiedHeartbeat,
					))

					cachedResults := store.InstanceHeartbeatCache()
					Expect(cachedResults).To(HaveLen(1))
					Expect(cachedResults[store.AppKey(dea.GetApp(1).AppGuid, dea.GetApp(1).AppVersion)]).To(ContainElement(modifiedHeartbeat))

					results, err := store.GetStoredInstanceHeartbeats()
					Expect(err).ToNot(HaveOccurred())
					Expect(results).To(HaveLen(1))
					Expect(results).To(ContainElement(modifiedHeartbeat))
				})

				Context("when the dea heartbeats and we do not receive any instances", func() {
					It("removes the old instance heartbeats from the cache and store", func() {
						store.SyncHeartbeats(dea.EmptyHeartbeat())

						cachedResults := store.InstanceHeartbeatCache()
						Expect(cachedResults[dea.GetApp(0).AppGuid+","+dea.GetApp(0).AppVersion]).To(HaveLen(0))

						results, err := store.GetStoredInstanceHeartbeats()
						Expect(err).ToNot(HaveOccurred())
						Expect(results).To(HaveLen(0))
					})

					It("remove the old instance heartbeat cache key if there are no more app instaces", func() {
						store.SyncHeartbeats(dea.EmptyHeartbeat())

						cachedResults := store.InstanceHeartbeatCache()
						Expect(cachedResults).To(HaveLen(0))
					})
				})

				It("Updates the expiration time for the cached dea", func() {
					results := store.DeaHeartbeatCache()
					Expect(results).To(HaveLen(1))
					Expect(results).To(HaveKey(dea.DeaGuid))

					initialTime := results[dea.DeaGuid]

					store.SyncHeartbeats(dea.EmptyHeartbeat())

					updatedResults := store.DeaHeartbeatCache()
					Expect(updatedResults).To(HaveLen(1))
					Expect(updatedResults).To(HaveKey(dea.DeaGuid))

					Expect(initialTime).To(BeNumerically("<", updatedResults[dea.DeaGuid]))
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

					results, err := store.GetStoredInstanceHeartbeats()
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(5))

					results, err = store.GetCachedInstanceHeartbeats()
					Expect(err).ToNot(HaveOccurred())
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

					results, err := store.GetStoredInstanceHeartbeats()
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

				It("Updates the expiration time for the received cached DEAs", func() {
					results := store.GetCachedDeaHeartbeats()
					Expect(results).To(HaveLen(3))
					Expect(results).To(HaveKey(dea.DeaGuid))
					Expect(results).To(HaveKey(otherDea.DeaGuid))
					Expect(results).To(HaveKey(yetAnotherDea.DeaGuid))

					deaTime := results[dea.DeaGuid]
					otherDeaTime := results[otherDea.DeaGuid]
					yetAnotherDeaTime := results[yetAnotherDea.DeaGuid]

					updateHeartbeat()

					updatedResults := store.GetCachedDeaHeartbeats()
					Expect(updatedResults).To(HaveLen(3))
					Expect(updatedResults).To(HaveKey(dea.DeaGuid))
					Expect(updatedResults).To(HaveKey(otherDea.DeaGuid))
					Expect(updatedResults).To(HaveKey(yetAnotherDea.DeaGuid))

					Expect(deaTime).To(BeNumerically("<", updatedResults[dea.DeaGuid]))
					Expect(otherDeaTime).To(BeNumerically("<", updatedResults[otherDea.DeaGuid]))
					Expect(yetAnotherDeaTime).To(Equal(updatedResults[yetAnotherDea.DeaGuid]))
				})
			})
		})
	})

	Describe(".GetCachedInstanceHeartbeats", func() {
		var (
			app1 appfixture.AppFixture
		)

		Context("when there are no cached instances", func() {
			It("returns an empty slice of instance heartbeats", func() {
				results, err := store.GetCachedInstanceHeartbeats()
				Expect(err).ToNot(HaveOccurred())
				Expect(len(results)).To(Equal(0))
				Expect(len(store.InstanceHeartbeatCache())).To(Equal(0))
			})
		})

		Context("when there are cached instances", func() {
			JustBeforeEach(func() {
				err := populate(dea.HeartbeatWith(
					dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
				))
				Expect(err).NotTo(HaveOccurred())

				app1 = dea.GetApp(0)
				store.AddDeaHeartbeats([]string{dea.DeaGuid})
			})

			It("returns the list of cached instance heartbeats", func() {
				err := store.EnsureCacheIsReady()
				Expect(err).NotTo(HaveOccurred())

				results, err := store.GetCachedInstanceHeartbeats()
				Expect(err).ToNot(HaveOccurred())

				Expect(len(results)).To(Equal(1))
				Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))

				Expect(len(store.InstanceHeartbeatCache())).To(Equal(1))
				Expect(store.InstanceHeartbeatCache()[dea.GetApp(0).AppGuid+","+dea.GetApp(0).AppVersion]).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
			})

			Context("when a single DEA has expired", func() {
				var (
					otherDea = appfixture.NewDeaFixture()

					constantApp appfixture.AppFixture
					expireApp   appfixture.AppFixture
				)

				JustBeforeEach(func() {
					err := populate(otherDea.HeartbeatWith(
						otherDea.GetApp(0).InstanceAtIndex(0).Heartbeat(),
					))
					Expect(err).NotTo(HaveOccurred())

					err = store.EnsureCacheIsReady()
					Expect(err).NotTo(HaveOccurred())

					conf.HeartbeatPeriod = 0
					store.AddDeaHeartbeats([]string{otherDea.DeaGuid})

					constantApp = dea.GetApp(0)
					expireApp = otherDea.GetApp(0)
				})

				It("returns apps from the valid DEAs", func() {
					results, err := store.GetCachedInstanceHeartbeats()
					Expect(err).ToNot(HaveOccurred())

					Expect(len(results)).To(Equal(1))
					Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))

					Expect(len(store.InstanceHeartbeatCache())).To(Equal(1))
					Expect(store.InstanceHeartbeatCache()[dea.GetApp(0).AppGuid+","+dea.GetApp(0).AppVersion]).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
				})

				It("It removes only the expired DEA's apps", func() {
					_, err := storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(constantApp.AppGuid, constantApp.AppVersion) + "/" + constantApp.InstanceAtIndex(1).Heartbeat().StoreKey())
					Expect(err).ToNot(HaveOccurred())
					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(expireApp.AppGuid, expireApp.AppVersion) + "/" + expireApp.InstanceAtIndex(0).Heartbeat().StoreKey())
					Expect(err).ToNot(HaveOccurred())

					results, err := store.GetCachedInstanceHeartbeats()
					Expect(err).ToNot(HaveOccurred())

					Expect(len(results)).To(Equal(1))
					Expect(results).To(ContainElement(constantApp.InstanceAtIndex(1).Heartbeat()))

					Expect(len(store.InstanceHeartbeatCache())).To(Equal(1))
					Expect(store.InstanceHeartbeatCache()[constantApp.AppGuid+","+constantApp.AppVersion]).To(ContainElement(constantApp.InstanceAtIndex(1).Heartbeat()))

					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(constantApp.AppGuid, constantApp.AppVersion) + "/" + constantApp.InstanceAtIndex(1).Heartbeat().StoreKey())
					Expect(err).ToNot(HaveOccurred())
					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(expireApp.AppGuid, expireApp.AppVersion) + "/" + expireApp.InstanceAtIndex(0).Heartbeat().StoreKey())
					Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
				})
			})

			Context("when all DEAs are expired", func() {
				JustBeforeEach(func() {
					err := store.EnsureCacheIsReady()
					Expect(err).NotTo(HaveOccurred())

					conf.HeartbeatPeriod = 0
					store.AddDeaHeartbeats([]string{dea.DeaGuid})
				})

				It("does not return any of the instances", func() {
					results, err := store.GetCachedInstanceHeartbeats()
					Expect(err).ToNot(HaveOccurred())
					Expect(len(results)).To(Equal(0))
					Expect(len(store.InstanceHeartbeatCache())).To(Equal(0))
				})

				It("remove the instances from the store", func() {
					_, err := storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(app1.AppGuid, app1.AppVersion) + "/" + app1.InstanceAtIndex(1).Heartbeat().StoreKey())
					Expect(err).ToNot(HaveOccurred())

					results, err := store.GetCachedInstanceHeartbeats()
					Expect(len(results)).To(Equal(0))
					Expect(err).ToNot(HaveOccurred())
					Expect(len(store.InstanceHeartbeatCache())).To(Equal(0))

					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(app1.AppGuid, app1.AppVersion) + "/" + app1.InstanceAtIndex(1).Heartbeat().StoreKey())
					Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
				})
			})

			Context("errors", func() {
				Context("when deleting from the store", func() {
					Context("when a delete returns ErrorKeyNotFound", func() {
						JustBeforeEach(func() {
							err := store.EnsureCacheIsReady()
							Expect(err).NotTo(HaveOccurred())

							conf.HeartbeatPeriod = 0
							store.AddDeaHeartbeats([]string{dea.DeaGuid})
						})

						It("does not return an error if we receive ErrorKeyNotFound and we delete the key from the cache", func() {
							err := storeAdapter.Delete("/hm/v1/apps/actual/" + store.AppKey(app1.AppGuid, app1.AppVersion) + "/" + app1.InstanceAtIndex(1).Heartbeat().StoreKey())
							Expect(err).ToNot(HaveOccurred())

							results, err := store.GetCachedInstanceHeartbeats()
							Expect(err).ToNot(HaveOccurred())
							Expect(len(results)).To(Equal(0))
							Expect(len(store.InstanceHeartbeatCache())).To(Equal(0))
						})
					})

					Context("all other errors", func() {
						var (
							expectedInstanceHeartbeats = map[string]InstanceHeartbeats{
								"app-guid,app-version": InstanceHeartbeats{
									"instance-guid": models.InstanceHeartbeat{AppGuid: "app-guid", DeaGuid: "dea-guid"},
								},
							}
						)

						JustBeforeEach(func() {
							fakeStoreAdapter := &fakes.FakeStoreAdapter{}
							fakeStoreAdapter.DeleteReturns(errors.New("wops"))
							storeAdapter = fakeStoreAdapter
							store = NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())

							store.SetInstanceHeartbeatCache(expectedInstanceHeartbeats)

							conf.HeartbeatPeriod = 0
							store.AddDeaHeartbeats([]string{"dea-guid"})
						})

						It("returns all other errors and does not delete the instance cache", func() {
							results, err := store.GetCachedInstanceHeartbeats()
							Expect(err).To(HaveOccurred())
							Expect(len(results)).To(Equal(0))
							Expect(len(store.InstanceHeartbeatCache())).To(Equal(1))

							Expect(store.InstanceHeartbeatCache()).To(Equal(expectedInstanceHeartbeats))
						})
					})
				})
			})
		})
	})

	Describe(".GetCachedInstanceHeartbeatsForApp", func() {
		Context("where there are no expired Instance Heartbeats", func() {
			var (
				otherApp appfixture.AppFixture
				app1     appfixture.AppFixture
			)

			JustBeforeEach(func() {
				err := populate(dea.HeartbeatWith(
					dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					dea.GetApp(0).InstanceAtIndex(2).Heartbeat(),
					dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
				))
				Expect(err).NotTo(HaveOccurred())

				app1 = dea.GetApp(0)
				otherApp = dea.GetApp(1)
				store.AddDeaHeartbeats([]string{dea.DeaGuid})

				err = store.EnsureCacheIsReady()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns the cached instance heartbeats for that app guid, app version", func() {
				results, err := store.GetCachedInstanceHeartbeatsForApp(app1.AppGuid, app1.AppVersion)
				Expect(err).NotTo(HaveOccurred())

				Expect(len(results)).To(Equal(2))
				Expect(results).ToNot(ContainElement(otherApp.InstanceAtIndex(3).Heartbeat()))
			})
		})

		Context("when there are expired Instance Heartbeats", func() {
			var (
				app1 appfixture.AppFixture
				app2 appfixture.AppFixture
				dea2 appfixture.DeaFixture
			)
			Context("when there are no errors", func() {
				JustBeforeEach(func() {
					dea2 = appfixture.NewDeaFixture()

					app1 = dea.GetApp(0)
					app2 = dea2.GetApp(1)
					app2.AppGuid = app1.AppGuid
					app2.AppVersion = app1.AppVersion

					err := populate(
						dea.HeartbeatWith(
							app1.InstanceAtIndex(1).Heartbeat(),
							app1.InstanceAtIndex(2).Heartbeat(),
						),
						dea2.HeartbeatWith(
							app2.InstanceAtIndex(0).Heartbeat(),
						),
					)
					Expect(err).NotTo(HaveOccurred())

					store.AddDeaHeartbeats([]string{dea2.DeaGuid})
					err = store.EnsureCacheIsReady()
					Expect(err).ToNot(HaveOccurred())

					conf.HeartbeatPeriod = 0
					store.AddDeaHeartbeats([]string{dea.DeaGuid})
				})

				It("does not return the expired heartbeats", func() {
					results, err := store.GetCachedInstanceHeartbeatsForApp(app1.AppGuid, app1.AppVersion)
					Expect(err).ToNot(HaveOccurred())

					Expect(len(results)).To(Equal(1))
					Expect(results).To(ContainElement(app2.InstanceAtIndex(0).Heartbeat()))
				})

				It("deletes the heartbeats from the expired dea from the cache", func() {
					_, err := store.GetCachedInstanceHeartbeatsForApp(app1.AppGuid, app1.AppVersion)
					Expect(err).ToNot(HaveOccurred())

					cachedInstanceHeartbeats := store.InstanceHeartbeatCache()
					Expect(len(cachedInstanceHeartbeats)).To(Equal(1))
					Expect(len(cachedInstanceHeartbeats[store.AppKey(app2.AppGuid, app2.AppVersion)])).To(Equal(1))
					Expect(cachedInstanceHeartbeats[store.AppKey(app2.AppGuid, app2.AppVersion)]).To(ContainElement(app2.InstanceAtIndex(0).Heartbeat()))
				})

				It("deletes the heartbeat from the expired dea from the store", func() {
					_, err := store.GetCachedInstanceHeartbeatsForApp(app1.AppGuid, app1.AppVersion)
					Expect(err).ToNot(HaveOccurred())

					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(app1.AppGuid, app1.AppVersion) + "/" + app1.InstanceAtIndex(1).Heartbeat().StoreKey())
					Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))

					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(app1.AppGuid, app1.AppVersion) + "/" + app1.InstanceAtIndex(2).Heartbeat().StoreKey())
					Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))

					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(app2.AppGuid, app2.AppVersion) + "/" + app2.InstanceAtIndex(0).Heartbeat().StoreKey())
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when deleting from the store causes error", func() {
				Context("when a delete returns ErrorKeyNotFound", func() {
					JustBeforeEach(func() {
						err := populate(dea.HeartbeatWith(
							dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
							dea.GetApp(0).InstanceAtIndex(2).Heartbeat(),
						))
						Expect(err).NotTo(HaveOccurred())

						app1 = dea.GetApp(0)

						err = store.EnsureCacheIsReady()
						Expect(err).ToNot(HaveOccurred())

						conf.HeartbeatPeriod = 0
						store.AddDeaHeartbeats([]string{dea.DeaGuid})
					})

					It("does not return an error if we receive ErrorKeyNotFound and we delete the key from the cache", func() {
						err := storeAdapter.Delete("/hm/v1/apps/actual/" + store.AppKey(app1.AppGuid, app1.AppVersion) + "/" + app1.InstanceAtIndex(1).Heartbeat().StoreKey())
						Expect(err).ToNot(HaveOccurred())

						results, err := store.GetCachedInstanceHeartbeatsForApp(app1.AppGuid, app1.AppVersion)
						Expect(err).ToNot(HaveOccurred())
						Expect(len(results)).To(Equal(0))
						Expect(len(store.InstanceHeartbeatCache())).To(Equal(0))
					})
				})

				Context("all other errors", func() {
					var (
						expectedInstanceHeartbeats = map[string]InstanceHeartbeats{
							"app-guid,app-version": InstanceHeartbeats{
								"instance-guid": models.InstanceHeartbeat{AppGuid: "app-guid", DeaGuid: "dea-guid"},
							},
						}
					)

					JustBeforeEach(func() {
						fakeStoreAdapter := &fakes.FakeStoreAdapter{}
						fakeStoreAdapter.DeleteReturns(errors.New("wops"))
						storeAdapter = fakeStoreAdapter
						store = NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())

						store.SetInstanceHeartbeatCache(expectedInstanceHeartbeats)

						conf.HeartbeatPeriod = 0
						store.AddDeaHeartbeats([]string{"dea-guid"})
					})

					It("returns all other errors and does not delete the instance cache", func() {
						results, err := store.GetCachedInstanceHeartbeatsForApp("app-guid", "app-version")
						Expect(err).To(HaveOccurred())
						Expect(len(results)).To(Equal(0))
						Expect(len(store.InstanceHeartbeatCache())).To(Equal(1))

						Expect(store.InstanceHeartbeatCache()).To(Equal(expectedInstanceHeartbeats))
					})
				})
			})

			Context("when all the heartbeats are from expired deas", func() {
				JustBeforeEach(func() {
					dea2 = appfixture.NewDeaFixture()

					app1 = dea.GetApp(0)
					app2 = dea2.GetApp(1)
					app2.AppGuid = app1.AppGuid
					app2.AppVersion = app1.AppVersion

					err := populate(
						dea.HeartbeatWith(
							app1.InstanceAtIndex(1).Heartbeat(),
							app1.InstanceAtIndex(2).Heartbeat(),
						),
						dea2.HeartbeatWith(
							app2.InstanceAtIndex(0).Heartbeat(),
						),
					)
					Expect(err).NotTo(HaveOccurred())

					err = store.EnsureCacheIsReady()
					Expect(err).ToNot(HaveOccurred())

					conf.HeartbeatPeriod = 0
					store.AddDeaHeartbeats([]string{dea.DeaGuid})

					store.AddDeaHeartbeats([]string{dea2.DeaGuid})
				})

				It("removes the instanceHeartbetCache key from the instanceHeartbetCache", func() {
					results, err := store.GetCachedInstanceHeartbeatsForApp(app1.AppGuid, app1.AppVersion)
					Expect(err).ToNot(HaveOccurred())
					Expect(len(results)).To(Equal(0))

					Expect(len(store.InstanceHeartbeatCache())).To(Equal(0))
				})
			})
		})
	})

	Describe(".GetCachedDeaHeartbeats", func() {
		Context("when there are no expired DEAs", func() {
			JustBeforeEach(func() {
				err := store.EnsureCacheIsReady()
				Expect(err).NotTo(HaveOccurred())

				store.AddDeaHeartbeats([]string{dea.DeaGuid})
			})

			It("returns the cached heartbeats", func() {
				results := store.GetCachedDeaHeartbeats()
				Expect(len(results)).To(Equal(1))

				Expect(results).To(HaveKey(dea.DeaGuid))
			})
		})

		Context("when there are expired DEAs", func() {
			BeforeEach(func() {
				conf.HeartbeatPeriod = 0
			})

			JustBeforeEach(func() {
				store.AddDeaHeartbeats([]string{dea.DeaGuid})
			})

			It("removes the expired dea from the dea cache", func() {
				results := store.GetCachedDeaHeartbeats()
				Expect(len(results)).To(Equal(0))
			})
		})
	})

	Describe(".AddDeaHeartbeats", func() {
		BeforeEach(func() {
			conf.HeartbeatPeriod = 1000
		})

		It("adds a future expiration time for all heartbeats", func() {
			heartbeatGuids := []string{"guid-1", "guid-2"}
			store.AddDeaHeartbeats(heartbeatGuids)

			cachedDeaHeartbeats := store.GetCachedDeaHeartbeats()
			Expect(len(cachedDeaHeartbeats)).To(Equal(2))
			Expect(cachedDeaHeartbeats["guid-1"]).To(BeNumerically(">", time.Now().UnixNano()))
			Expect(cachedDeaHeartbeats["guid-2"]).To(BeNumerically(">", time.Now().UnixNano()))
		})
	})
})
