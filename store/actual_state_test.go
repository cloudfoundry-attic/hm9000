package store_test

import (
	. "github.com/cloudfoundry/hm9000/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
)

var _ = Describe("Actual State", func() {
	var (
		store       Store
		storeAdapter storeadapter.StoreAdapter
		conf        config.Config
		dea         appfixture.DeaFixture
		otherDea    appfixture.DeaFixture
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())
		storeAdapter = storeadapter.NewETCDStoreAdapter(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err = storeAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())
		store = NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())

		dea = appfixture.NewDeaFixture()
		otherDea = appfixture.NewDeaFixture()
	})

	AfterEach(func() {
		storeAdapter.Disconnect()
	})

	Describe("Saving actual state", func() {
		BeforeEach(func() {
			store.SyncHeartbeat(dea.HeartbeatWith(
				dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
				dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
			))
		})

		It("should save the instance heartbeats for the passed-in heartbeat", func() {
			results, err := store.GetInstanceHeartbeats()
			Ω(err).ShouldNot(HaveOccured())
			Ω(results).Should(HaveLen(2))
			Ω(results).Should(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
			Ω(results).Should(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))
		})

		Context("when there are already instance heartbeats stored for the DEA in question", func() {
			var modifiedHeartbeat models.InstanceHeartbeat
			BeforeEach(func() {
				modifiedHeartbeat = dea.GetApp(1).InstanceAtIndex(3).Heartbeat()
				modifiedHeartbeat.State = models.InstanceStateEvacuating
				store.SyncHeartbeat(dea.HeartbeatWith(
					modifiedHeartbeat,
					dea.GetApp(2).InstanceAtIndex(2).Heartbeat(),
				))
			})

			It("should sync the heartbeats (add new ones, adjust ones that have changed state, and delete old ones)", func() {
				results, err := store.GetInstanceHeartbeats()
				Ω(err).ShouldNot(HaveOccured())
				Ω(results).Should(HaveLen(2))
				Ω(results).Should(ContainElement(modifiedHeartbeat))
				Ω(results).Should(ContainElement(dea.GetApp(2).InstanceAtIndex(2).Heartbeat()))
			})
		})
	})

	Describe("Fetching all actual state", func() {
		Context("when there is none saved", func() {
			It("should come back empty", func() {
				results, err := store.GetInstanceHeartbeats()
				Ω(err).ShouldNot(HaveOccured())
				Ω(results).Should(BeEmpty())
			})
		})

		Context("when there is actual state saved", func() {
			BeforeEach(func() {
				store.SyncHeartbeat(dea.HeartbeatWith(
					dea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					dea.GetApp(1).InstanceAtIndex(3).Heartbeat(),
				))

				store.SyncHeartbeat(otherDea.HeartbeatWith(
					otherDea.GetApp(0).InstanceAtIndex(1).Heartbeat(),
					otherDea.GetApp(1).InstanceAtIndex(0).Heartbeat(),
				))
			})

			Context("when the DEA heartbeats have not expired", func() {
				It("should return the instance heartbeats", func() {
					results, err := store.GetInstanceHeartbeats()
					Ω(err).ShouldNot(HaveOccured())
					Ω(results).Should(HaveLen(4))
					Ω(results).Should(ContainElement(dea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Ω(results).Should(ContainElement(dea.GetApp(1).InstanceAtIndex(3).Heartbeat()))
					Ω(results).Should(ContainElement(otherDea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Ω(results).Should(ContainElement(otherDea.GetApp(1).InstanceAtIndex(0).Heartbeat()))
				})
			})

			Context("when a DEA heartbeat has expired", func() {
				BeforeEach(func() {
					storeAdapter.Delete("/dea-presence/" + dea.DeaGuid)
				})

				It("should not return any expired instance heartbeats", func() {
					results, err := store.GetInstanceHeartbeats()
					Ω(err).ShouldNot(HaveOccured())
					Ω(results).Should(HaveLen(2))
					Ω(results).Should(ContainElement(otherDea.GetApp(0).InstanceAtIndex(1).Heartbeat()))
					Ω(results).Should(ContainElement(otherDea.GetApp(1).InstanceAtIndex(0).Heartbeat()))
				})

				It("should remove expired instance heartbeats from the store", func() {
					_, err := storeAdapter.Get("/apps/actual/" + store.AppKey(dea.GetApp(0).AppGuid, dea.GetApp(0).AppVersion) + "/" + dea.GetApp(0).InstanceAtIndex(1).Heartbeat().StoreKey())
					Ω(err).ShouldNot(HaveOccured())
					_, err = storeAdapter.Get("/apps/actual/" + store.AppKey(dea.GetApp(1).AppGuid, dea.GetApp(1).AppVersion) + "/" + dea.GetApp(1).InstanceAtIndex(3).Heartbeat().StoreKey())
					Ω(err).ShouldNot(HaveOccured())

					_, err = store.GetInstanceHeartbeats()
					Ω(err).ShouldNot(HaveOccured())

					_, err = storeAdapter.Get("/apps/actual/" + store.AppKey(dea.GetApp(0).AppGuid, dea.GetApp(0).AppVersion) + "/" + dea.GetApp(0).InstanceAtIndex(1).Heartbeat().StoreKey())
					Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))
					_, err = storeAdapter.Get("/apps/actual/" + store.AppKey(dea.GetApp(1).AppGuid, dea.GetApp(1).AppVersion) + "/" + dea.GetApp(1).InstanceAtIndex(3).Heartbeat().StoreKey())
					Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))
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
				Ω(err).ShouldNot(HaveOccured())
				Ω(results).Should(BeEmpty())
			})
		})

		Context("when there is actual state saved", func() {
			var heartbeatA, heartbeatB models.InstanceHeartbeat

			BeforeEach(func() {
				heartbeatA = app.InstanceAtIndex(0).Heartbeat()
				heartbeatA.DeaGuid = "A"

				store.SyncHeartbeat(models.Heartbeat{
					DeaGuid: "A",
					InstanceHeartbeats: []models.InstanceHeartbeat{
						heartbeatA,
					},
				})

				heartbeatB = app.InstanceAtIndex(1).Heartbeat()
				heartbeatB.DeaGuid = "B"

				store.SyncHeartbeat(models.Heartbeat{
					DeaGuid: "B",
					InstanceHeartbeats: []models.InstanceHeartbeat{
						heartbeatB,
					},
				})
			})

			Context("when the corresponding DEA heartbeat has not expired", func() {
				It("should return the instance heartbeats", func() {
					results, err := store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Ω(err).ShouldNot(HaveOccured())
					Ω(results).Should(HaveLen(2))
					Ω(results).Should(ContainElement(heartbeatA))
					Ω(results).Should(ContainElement(heartbeatB))
				})
			})

			Context("when the corresponding DEA heartbeat has expired", func() {
				BeforeEach(func() {
					storeAdapter.Delete("/dea-presence/A")
				})

				It("should not return any expired instance heartbeats", func() {
					results, err := store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Ω(err).ShouldNot(HaveOccured())
					Ω(results).Should(HaveLen(1))
					Ω(results).Should(ContainElement(heartbeatB))
				})

				It("should remove expired instance heartbeats from the store", func() {
					_, err := storeAdapter.Get("/apps/actual/" + store.AppKey(app.AppGuid, app.AppVersion) + "/" + heartbeatA.StoreKey())
					Ω(err).ShouldNot(HaveOccured())

					_, err = store.GetInstanceHeartbeatsForApp(app.AppGuid, app.AppVersion)
					Ω(err).ShouldNot(HaveOccured())

					_, err = storeAdapter.Get("/apps/actual/" + store.AppKey(app.AppGuid, app.AppVersion) + "/" + heartbeatA.StoreKey())
					Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))
				})
			})
		})
	})
})
