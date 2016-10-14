package store_test

import (
	"errors"

	"code.cloudfoundry.org/workpool"
	. "github.com/cloudfoundry/hm9000/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/fakes"
)

var _ = Describe("Actual State", func() {
	var (
		store        Store
		storeAdapter storeadapter.StoreAdapter
		conf         *config.Config
		dea          appfixture.DeaFixture
		otherDea     appfixture.DeaFixture
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
	})

	AfterEach(func() {
		storeAdapter.Disconnect()
	})

	Describe("Saving actual state", func() {
		BeforeEach(func() {
			store.SyncHeartbeats(dea.HeartbeatWith(
				dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
				dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
			))
		})

		It("should save the instance heartbeats for the passed-in heartbeat", func() {
			results, err := store.GetInstanceHeartbeats()
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(2))
			Expect(results).To(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
			Expect(results).To(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))
		})

		Context("when there are already instance heartbeats stored for the DEA in question", func() {
			var modifiedHeartbeat models.InstanceHeartbeat
			BeforeEach(func() {
				modifiedHeartbeat = dea.GetApp(1).InstanceAtIndex(3).Heartbeat()
				modifiedHeartbeat.State = models.InstanceStateEvacuating
				store.SyncHeartbeats(dea.HeartbeatWith(
					modifiedHeartbeat,
					dea.GetApp(2).InstanceAtIndex(2).Heartbeat(),
				))
			})

			It("should sync the heartbeats (add new ones, adjust ones that have changed state, and delete old ones)", func() {
				results, err := store.GetInstanceHeartbeats()
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(2))
				Expect(results).To(ContainElement(modifiedHeartbeat))
				Expect(results).To(ContainElement(dea.GetApp(2).InstanceAtIndex(2).Heartbeat()))
			})
		})

		Context("when saving multiple heartbeats at once", func() {
			var modifiedHeartbeat models.InstanceHeartbeat
			var yetAnotherDea appfixture.DeaFixture

			BeforeEach(func() {
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

				modifiedHeartbeat = dea.GetApp(1).InstanceAtIndex(3).Heartbeat()
				modifiedHeartbeat.State = models.InstanceStateEvacuating
				store.SyncHeartbeats(dea.HeartbeatWith(
					modifiedHeartbeat,
					dea.GetApp(2).InstanceAtIndex(2).Heartbeat(),
				), otherDea.HeartbeatWith(
					otherDea.GetApp(2).InstanceAtIndex(1).Heartbeat(),
					otherDea.GetApp(3).InstanceAtIndex(2).Heartbeat(),
				))
			})

			It("should work", func() {
				results, err := store.GetInstanceHeartbeats()
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(5))
				Expect(results).To(ContainElement(modifiedHeartbeat))
				Expect(results).To(ContainElement(dea.GetApp(2).InstanceAtIndex(2).Heartbeat()))
				Expect(results).To(ContainElement(otherDea.GetApp(2).InstanceAtIndex(1).Heartbeat()))
				Expect(results).To(ContainElement(otherDea.GetApp(3).InstanceAtIndex(2).Heartbeat()))
				Expect(results).To(ContainElement(yetAnotherDea.GetApp(0).InstanceAtIndex(0).Heartbeat()))
			})
		})

		Context("when one of the keys fails to delete", func() {
			It("should soldier on", func() {
				store.SyncHeartbeats(dea.HeartbeatWith(
					dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
				))

				done := make(chan error, 2)

				go func() {
					_, err := store.SyncHeartbeats(dea.HeartbeatWith(
						dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					))
					done <- err
				}()

				go func() {
					_, err := store.SyncHeartbeats(dea.HeartbeatWith(
						dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					))
					done <- err
				}()

				err1 := <-done
				err2 := <-done
				Expect(err1).NotTo(HaveOccurred())
				Expect(err2).NotTo(HaveOccurred())
			})
		})

		Context("when something goes wrong and the in-memory cache no longer matches the store", func() {
			It("should eventually recover", func() {
				//Delete one of the heartbeats
				corruptedHeartbeat := dea.GetApp(0).InstanceAtIndex(1).Heartbeat()
				storeAdapter.Delete("/hm/v1/apps/actual/" + store.AppKey(corruptedHeartbeat.AppGuid, corruptedHeartbeat.AppVersion) + "/" + corruptedHeartbeat.InstanceGuid)

				//See that it's gone
				results, err := store.GetInstanceHeartbeats()
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(1))

				//Try to put it back
				store.SyncHeartbeats(dea.HeartbeatWith(
					dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
				))

				//See that we didn't... because it's still in the cache...
				results, err = store.GetInstanceHeartbeats()
				Expect(err).NotTo(HaveOccurred())
				Expect(results).To(HaveLen(1))

				//Eventually the cache should be reloaded...
				Eventually(func() []models.InstanceHeartbeat {
					store.SyncHeartbeats(dea.HeartbeatWith(
						dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
						dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
					))

					results, err = store.GetInstanceHeartbeats()
					Expect(err).NotTo(HaveOccurred())
					return results
				}, 1.0, 0.05).Should(HaveLen(2)) //...and the heartbeat should return
			})
		})

		Context("When a Heartbeat contains Instance heartbeats in the Evacuating state", func() {
			var (
				err                    error
				evacuatingHeartbeat    models.InstanceHeartbeat
				notEvacuatingHeartbeat models.InstanceHeartbeat
				heartbeatsToEvac       []models.InstanceHeartbeat
				crashedHeartbeat       models.InstanceHeartbeat
			)

			BeforeEach(func() {
				evacuatingHeartbeat = dea.GetApp(1).InstanceAtIndex(3).Heartbeat()
				evacuatingHeartbeat.State = models.InstanceStateEvacuating
				notEvacuatingHeartbeat = dea.GetApp(2).InstanceAtIndex(2).Heartbeat()
				crashedHeartbeat = dea.GetApp(1).InstanceAtIndex(3).Heartbeat()
				crashedHeartbeat.State = models.InstanceStateCrashed
			})

			It("does return the heartbeats if the cached is empty", func() {
				newEvacuatingHeartbeat := dea.GetApp(2).InstanceAtIndex(0).Heartbeat()
				newEvacuatingHeartbeat.State = models.InstanceStateEvacuating

				heartbeatsToEvac, err = store.SyncHeartbeats(dea.HeartbeatWith(
					newEvacuatingHeartbeat,
				))
				Expect(err).NotTo(HaveOccurred())
				Expect(len(heartbeatsToEvac)).To(Equal(1))
				Expect(heartbeatsToEvac).To(ContainElement(newEvacuatingHeartbeat))
			})

			Context("when the the instance states are in the running state", func() {
				BeforeEach(func() {
					heartbeatsToEvac, err = store.SyncHeartbeats(dea.HeartbeatWith(
						evacuatingHeartbeat,
						notEvacuatingHeartbeat,
					))
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the instance heartbeats that should send a start message", func() {
					Expect(len(heartbeatsToEvac)).To(Equal(1))
					Expect(heartbeatsToEvac).To(ContainElement(evacuatingHeartbeat))
				})

				It("does not return the heartbeat in the evacuating state", func() {
					Expect(heartbeatsToEvac).ToNot(ContainElement(notEvacuatingHeartbeat))
				})
			})

			Context("when the instance states are in the CRASHED state", func() {
				BeforeEach(func() {
					heartbeatsToEvac, err = store.SyncHeartbeats(dea.HeartbeatWith(
						crashedHeartbeat,
						notEvacuatingHeartbeat,
					))
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not return the heartbeat for evacuation", func() {
					heartbeatsToEvac, err = store.SyncHeartbeats(dea.HeartbeatWith(
						evacuatingHeartbeat,
						notEvacuatingHeartbeat,
					))
					Expect(err).NotTo(HaveOccurred())
					Expect(len(heartbeatsToEvac)).To(Equal(0))
					Expect(heartbeatsToEvac).ToNot(ContainElement(evacuatingHeartbeat))
				})
			})

			Context("when a second request comes in and the instance states are already evacuating", func() {
				BeforeEach(func() {
					heartbeatsToEvac, err = store.SyncHeartbeats(dea.HeartbeatWith(
						evacuatingHeartbeat,
						notEvacuatingHeartbeat,
					))
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not return the instance heartbeats", func() {
					heartbeatsToEvac, err := store.SyncHeartbeats(dea.HeartbeatWith(
						evacuatingHeartbeat,
						notEvacuatingHeartbeat,
					))

					Expect(err).NotTo(HaveOccurred())
					Expect(len(heartbeatsToEvac)).To(Equal(0))
				})
			})
		})
	})

	Describe("Fetching all actual state", func() {
		Context("when there is none saved", func() {
			It("should come back empty", func() {
				results, err := store.GetInstanceHeartbeats()
				Expect(err).NotTo(HaveOccurred())
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
			})

			Context("when the DEA heartbeats have not expired", func() {
				BeforeEach(func() {
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

				It("should return the instance heartbeats", func() {
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

					storeAdapter.Delete("/hm/v1/dea-presence/" + dea.DeaGuid)
				})

				It("should not return any expired instance heartbeats", func() {
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

				It("should remove expired instance heartbeats from the store", func() {
					_, err := storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(dea.GetApp(0).AppGuid, dea.GetApp(0).AppVersion) + "/" + dea.GetApp(0).InstanceAtIndex(1).Heartbeat().StoreKey())
					Expect(err).NotTo(HaveOccurred())
					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(dea.GetApp(1).AppGuid, dea.GetApp(1).AppVersion) + "/" + dea.GetApp(1).InstanceAtIndex(3).Heartbeat().StoreKey())
					Expect(err).NotTo(HaveOccurred())

					_, err = store.GetInstanceHeartbeats()
					Expect(err).NotTo(HaveOccurred())

					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(dea.GetApp(0).AppGuid, dea.GetApp(0).AppVersion) + "/" + dea.GetApp(0).InstanceAtIndex(1).Heartbeat().StoreKey())
					Expect(err.(storeadapter.Error).Type()).To(Equal(storeadapter.ErrorKeyNotFound))
					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(dea.GetApp(1).AppGuid, dea.GetApp(1).AppVersion) + "/" + dea.GetApp(1).InstanceAtIndex(3).Heartbeat().StoreKey())
					Expect(err.(storeadapter.Error).Type()).To(Equal(storeadapter.ErrorKeyNotFound))
				})
			})

			Context("failures", func() {
				var fakeStoreAdapter *fakes.FakeStoreAdapter

				Context("When the storeadapter does not have a key for an app", func() {
					BeforeEach(func() {
						fakeStoreAdapter = &fakes.FakeStoreAdapter{}
						fakeStoreAdapter.DeleteReturns(storeadapter.NewError(errors.New(storeadapter.ErrorKeyNotFoundDescription), storeadapter.ErrorKeyNotFound))

						store = NewStore(conf, fakeStoreAdapter, fakelogger.NewFakeLogger())
					})

					It("does not return an error", func() {
						_, err := store.GetInstanceHeartbeats()
						Expect(err).NotTo(HaveOccurred())
						Expect(fakeStoreAdapter.DeleteCallCount()).To(Equal(1))
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
			It("should come back empty", func() {
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
				It("should return the instance heartbeats", func() {
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

				It("should not return any expired instance heartbeats", func() {
					results, err := store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(results).To(HaveLen(1))
					Expect(results).To(ContainElement(heartbeatB))
				})

				It("should remove expired instance heartbeats from the store", func() {
					_, err := storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(app.AppGuid, app.AppVersion) + "/" + heartbeatA.StoreKey())
					Expect(err).NotTo(HaveOccurred())

					_, err = store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Expect(err).NotTo(HaveOccurred())

					_, err = storeAdapter.Get("/hm/v1/apps/actual/" + store.AppKey(app.AppGuid, app.AppVersion) + "/" + heartbeatA.StoreKey())
					Expect(err.(storeadapter.Error).Type()).To(Equal(storeadapter.ErrorKeyNotFound))
				})
			})

			Context("when all the DEA heartbeats have expired", func() {
				BeforeEach(func() {
					storeAdapter.Delete("/hm/v1/dea-presence/A", "/hm/v1/dea-presence/B")
				})

				It("should not return any instance heartbeats", func() {
					results, err := store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(results).NotTo(BeNil())
					Expect(results).To(HaveLen(0))
				})
			})

			Context("failures", func() {
				var fakeStoreAdapter *fakes.FakeStoreAdapter

				Context("When the storeadapter does not have a key for an app", func() {
					BeforeEach(func() {
						fakeStoreAdapter = &fakes.FakeStoreAdapter{}
						fakeStoreAdapter.DeleteReturns(storeadapter.NewError(errors.New(storeadapter.ErrorKeyNotFoundDescription), storeadapter.ErrorKeyNotFound))

						store = NewStore(conf, fakeStoreAdapter, fakelogger.NewFakeLogger())
					})

					It("does not return an error", func() {
						_, err := store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
						Expect(err).NotTo(HaveOccurred())
						Expect(fakeStoreAdapter.DeleteCallCount()).To(Equal(1))
					})
				})
			})

		})
	})

	Describe("Updating the dea cache", func() {
		var fakeStoreAdapter *fakes.FakeStoreAdapter

		Context("when there are no DEAs in the etcd store", func() {
			It("returns an empty cache and no error", func() {
				deaCache, err := store.GetDeaCache()
				Expect(err).ToNot(HaveOccurred())
				Expect(deaCache).To(BeEmpty())
			})
		})

		Context("Errors", func() {
			BeforeEach(func() {
				fakeStoreAdapter = &fakes.FakeStoreAdapter{}

				store = NewStore(conf, fakeStoreAdapter, fakelogger.NewFakeLogger())
			})

			Context("when the etcd store cannot be reached", func() {
				BeforeEach(func() {
					fakeStoreAdapter.ListRecursivelyReturns(
						storeadapter.StoreNode{
							Key:   "key",
							Value: []byte("value"),
							Dir:   true,
							TTL:   10,
							Index: 1,
						},
						errors.New("bad request"),
					)
				})

				It("returns an empty cache and the error", func() {
					deaCache, err := store.GetDeaCache()
					Expect(err).To(HaveOccurred())
					Expect(deaCache).To(BeEmpty())
				})
			})

			Context("when the etcd store returns a key not found error", func() {
				BeforeEach(func() {
					fakeStoreAdapter.ListRecursivelyReturns(storeadapter.StoreNode{}, storeadapter.NewError(errors.New(storeadapter.ErrorKeyNotFoundDescription), storeadapter.ErrorKeyNotFound))
				})

				It("returns an empty cache and no error", func() {
					deaCache, err := store.GetDeaCache()
					Expect(err).ToNot(HaveOccurred())
					Expect(deaCache).To(BeEmpty())
				})
			})
		})

		Context("when there are DEAs in the etcd store", func() {
			var firstDeaCache map[string]struct{}

			BeforeEach(func() {
				app := appfixture.NewAppFixture()

				heartbeatA := app.InstanceAtIndex(0).Heartbeat()
				heartbeatA.DeaGuid = "A"

				store.SyncHeartbeats(&models.Heartbeat{
					DeaGuid: "A",
					InstanceHeartbeats: []models.InstanceHeartbeat{
						heartbeatA,
					},
				})

				heartbeatB := app.InstanceAtIndex(1).Heartbeat()
				heartbeatB.DeaGuid = "B"

				store.SyncHeartbeats(&models.Heartbeat{
					DeaGuid: "B",
					InstanceHeartbeats: []models.InstanceHeartbeat{
						heartbeatB,
					},
				})
			})

			JustBeforeEach(func() {
				var err error
				firstDeaCache, err = store.GetDeaCache()
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns all valid DEAs", func() {
				deaCache, err := store.GetDeaCache()
				Expect(err).ToNot(HaveOccurred())

				Expect(deaCache).To(HaveKey("A"))
				Expect(deaCache).To(HaveKey("B"))
			})

			Context("and a heartbeat interval has not passed", func() {
				BeforeEach(func() {
					conf.HeartbeatPeriod = 100000
				})

				It("does not update the cache on the next request", func() {
					secondDeaCache, err := store.GetDeaCache()
					Expect(err).ToNot(HaveOccurred())
					Expect(secondDeaCache).To(Equal(firstDeaCache))
				})
			})

			Context("and a heartbeat interval has passed", func() {
				Context("and a DEA has expired", func() {
					BeforeEach(func() {
						conf.HeartbeatPeriod = 0
					})

					JustBeforeEach(func() {
						storeAdapter.Delete("/hm/v1/dea-presence/A")
					})

					It("removes an expired DEA from the cache", func() {
						secondDeaCache, err := store.GetDeaCache()
						Expect(err).ToNot(HaveOccurred())
						Expect(secondDeaCache).ToNot(Equal(firstDeaCache))

						Expect(secondDeaCache).ToNot(HaveKey("A"))
						Expect(secondDeaCache).To(HaveKey("B"))
					})
				})
			})
		})
	})
})
