package store_test

import (
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Apps", func() {
	var (
		store        Store
		storeAdapter storeadapter.StoreAdapter
		conf         *config.Config

		dea        appfixture.DeaFixture
		app1       appfixture.AppFixture
		app2       appfixture.AppFixture
		app3       appfixture.AppFixture
		app4       appfixture.AppFixture
		crashCount []models.CrashCount
	)

	conf, _ = config.DefaultConfig()

	BeforeEach(func() {
		wpool, err := workpool.NewWorkPool(conf.StoreMaxConcurrentRequests)
		Expect(err).NotTo(HaveOccurred())
		storeAdapter, err = etcdstoreadapter.New(
			&etcdstoreadapter.ETCDOptions{ClusterUrls: etcdRunner.NodeURLS()},
			wpool,
		)
		Expect(err).NotTo(HaveOccurred())
		err = storeAdapter.Connect()
		Expect(err).NotTo(HaveOccurred())

		store = NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())

		dea = appfixture.NewDeaFixture()
		app1 = dea.GetApp(0)
		app2 = dea.GetApp(1)
		app3 = dea.GetApp(2)
		app4 = dea.GetApp(3)

		actualState := []models.InstanceHeartbeat{
			app1.InstanceAtIndex(0).Heartbeat(),
			app1.InstanceAtIndex(1).Heartbeat(),
			app1.InstanceAtIndex(2).Heartbeat(),
			app2.InstanceAtIndex(0).Heartbeat(),
		}

		desiredState := []models.DesiredAppState{
			app1.DesiredState(1),
			app3.DesiredState(1),
		}

		crashCount = []models.CrashCount{
			{
				AppGuid:       app1.AppGuid,
				AppVersion:    app1.AppVersion,
				InstanceIndex: 1,
				CrashCount:    12,
			},
			{
				AppGuid:       app1.AppGuid,
				AppVersion:    app1.AppVersion,
				InstanceIndex: 2,
				CrashCount:    17,
			},
			{
				AppGuid:       app2.AppGuid,
				AppVersion:    app2.AppVersion,
				InstanceIndex: 0,
				CrashCount:    3,
			},
			{
				AppGuid:       app4.AppGuid,
				AppVersion:    app4.AppVersion,
				InstanceIndex: 1,
				CrashCount:    8,
			},
		}

		store.SyncHeartbeats(dea.HeartbeatWith(actualState...))
		store.SyncDesiredState(desiredState...)
		store.SaveCrashCounts(crashCount...)
	})

	Describe("AppKey", func() {
		It("To concatenate the app guid and app version appropriately", func() {
			key := store.AppKey("abc", "123")
			Expect(key).To(Equal("abc,123"))
		})
	})

	Describe("GetApps", func() {
		Context("when all is well", func() {
			It("To build and return the set of apps", func() {
				apps, err := store.GetApps()
				Expect(err).NotTo(HaveOccurred())

				Expect(apps).To(HaveLen(3))
				Expect(apps).To(HaveKey(app1.AppGuid + "," + app1.AppVersion))
				Expect(apps).To(HaveKey(app2.AppGuid + "," + app2.AppVersion))
				Expect(apps).To(HaveKey(app3.AppGuid + "," + app3.AppVersion))

				a1 := apps[app1.AppGuid+","+app1.AppVersion]
				Expect(a1.Desired).To(EqualDesiredState(app1.DesiredState(1)))
				Expect(a1.InstanceHeartbeats).To(HaveLen(3))
				Expect(a1.InstanceHeartbeats).To(ContainElement(app1.InstanceAtIndex(0).Heartbeat()))
				Expect(a1.InstanceHeartbeats).To(ContainElement(app1.InstanceAtIndex(1).Heartbeat()))
				Expect(a1.InstanceHeartbeats).To(ContainElement(app1.InstanceAtIndex(2).Heartbeat()))
				Expect(a1.CrashCounts[1]).To(Equal(crashCount[0]))
				Expect(a1.CrashCounts[2]).To(Equal(crashCount[1]))

				a2 := apps[app2.AppGuid+","+app2.AppVersion]
				Expect(a2.Desired).To(BeZero())
				Expect(a2.InstanceHeartbeats).To(HaveLen(1))
				Expect(a2.InstanceHeartbeats).To(ContainElement(app2.InstanceAtIndex(0).Heartbeat()))
				Expect(a2.CrashCounts[0]).To(Equal(crashCount[2]))

				a3 := apps[app3.AppGuid+","+app3.AppVersion]
				Expect(a3.Desired).To(EqualDesiredState(app3.DesiredState(1)))
				Expect(a3.InstanceHeartbeats).To(HaveLen(0))
				Expect(a3.CrashCounts).To(BeEmpty())
			})
		})

		Context("when there is an empty app directory", func() {
			It("To ignore that app directory", func() {
				storeAdapter.SetMulti([]storeadapter.StoreNode{{
					Key:   "/hm/v1/apps/actual/foo-bar",
					Value: []byte("foo"),
				}})

				apps, err := store.GetApps()
				Expect(err).NotTo(HaveOccurred())
				Expect(apps).To(HaveLen(3))
			})
		})
	})

	Describe("GetApp", func() {
		Context("when there are no store errors", func() {
			Context("when the app has desired and actual state", func() {
				It("To return the app", func() {
					app, err := store.GetApp(app1.AppGuid, app1.AppVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(app.Desired).To(EqualDesiredState(app1.DesiredState(1)))
					Expect(app.InstanceHeartbeats).To(HaveLen(3))
					Expect(app.InstanceHeartbeats).To(ContainElement(app1.InstanceAtIndex(0).Heartbeat()))
					Expect(app.InstanceHeartbeats).To(ContainElement(app1.InstanceAtIndex(1).Heartbeat()))
					Expect(app.InstanceHeartbeats).To(ContainElement(app1.InstanceAtIndex(2).Heartbeat()))
					Expect(app.CrashCounts[1]).To(Equal(crashCount[0]))
					Expect(app.CrashCounts[2]).To(Equal(crashCount[1]))
				})
			})

			Context("when the app has desired state only", func() {
				It("To return the app", func() {
					app, err := store.GetApp(app3.AppGuid, app3.AppVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(app.Desired).To(EqualDesiredState(app3.DesiredState(1)))
					Expect(app.InstanceHeartbeats).To(BeEmpty())
					Expect(app.CrashCounts).To(BeEmpty())
				})
			})

			Context("when the app has actual state only", func() {
				It("To return the app", func() {
					app, err := store.GetApp(app2.AppGuid, app2.AppVersion)
					Expect(err).NotTo(HaveOccurred())
					Expect(app.Desired).To(BeZero())
					Expect(app.InstanceHeartbeats).To(HaveLen(1))
					Expect(app.InstanceHeartbeats).To(ContainElement(app2.InstanceAtIndex(0).Heartbeat()))
					Expect(app.CrashCounts[0]).To(Equal(crashCount[2]))
				})
			})

			Context("when the app has crash counts only", func() {
				It("To return the app not found error", func() {
					app, err := store.GetApp(app4.AppGuid, app4.AppVersion)
					Expect(err).To(Equal(AppNotFoundError))
					Expect(app).To(BeZero())
				})
			})

			Context("when the app is not found", func() {
				It("To return the app not found error", func() {
					app, err := store.GetApp("Marzipan", "Armadillo")
					Expect(err).To(Equal(AppNotFoundError))
					Expect(app).To(BeZero())
				})
			})

			Context("when the app directory is empty", func() {
				It("To return the app not found error", func() {
					storeAdapter.SetMulti([]storeadapter.StoreNode{{
						Key:   "/hm/v1/apps/actual/foo-bar/baz",
						Value: []byte("foo"),
					}})

					storeAdapter.Delete("/hm/v1/apps/actual/foo-bar/baz")

					app, err := store.GetApp("foo", "bar")
					Expect(err).To(Equal(AppNotFoundError))
					Expect(app).To(BeZero())
				})
			})

		})
	})
})
