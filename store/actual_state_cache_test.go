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

var _ = FDescribe("Cached actual state", func() {
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
	})

	JustBeforeEach(func() {
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

						BeforeEach(func() {
							fakeStoreAdapter := &fakes.FakeStoreAdapter{}
							fakeStoreAdapter.DeleteReturns(errors.New("wops"))
							storeAdapter = fakeStoreAdapter
						})

						JustBeforeEach(func() {
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

	Describe(".GetCachedInstanceHeartbeatsForApp", func() {

	})
})
