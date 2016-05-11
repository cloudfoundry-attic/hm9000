package store_test

import (
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
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

				Expect(apps).To(HaveLen(2))
				Expect(apps).To(HaveKey(app1.AppGuid + "," + app1.AppVersion))
				Expect(apps).To(HaveKey(app2.AppGuid + "," + app2.AppVersion))

				a1 := apps[app1.AppGuid+","+app1.AppVersion]
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
				Expect(apps).To(HaveLen(2))
			})
		})
	})

	Describe("GetApp", func() {
		Context("when there are no store errors", func() {
			It("To return the app, with an empty desired state", func() {
				app, err := store.GetApp(app2.AppGuid, app2.AppVersion)
				Expect(err).NotTo(HaveOccurred())
				Expect(app.Desired).To(BeZero())
				Expect(app.InstanceHeartbeats).To(HaveLen(1))
				Expect(app.InstanceHeartbeats).To(ContainElement(app2.InstanceAtIndex(0).Heartbeat()))
				Expect(app.CrashCounts[0]).To(Equal(crashCount[2]))
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
