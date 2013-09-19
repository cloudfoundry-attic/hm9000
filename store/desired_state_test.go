package store_test

import (
	. "github.com/cloudfoundry/hm9000/store"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
)

var _ = Describe("Desired State", func() {
	var (
		store       Store
		etcdAdapter storeadapter.StoreAdapter
		conf        config.Config
		app1        app.App
		app2        app.App
		app3        app.App
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())
		etcdAdapter = storeadapter.NewETCDStoreAdapter(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err = etcdAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())

		app1 = app.NewApp()
		app2 = app.NewApp()
		app3 = app.NewApp()

		store = NewStore(conf, etcdAdapter)
	})

	AfterEach(func() {
		etcdAdapter.Disconnect()
	})

	Describe("Saving desired state ", func() {
		BeforeEach(func() {
			err := store.SaveDesiredState([]models.DesiredAppState{
				app1.DesiredState(0),
				app2.DesiredState(0),
			})
			Ω(err).ShouldNot(HaveOccured())
		})

		It("can stores the passed in desired state", func() {
			nodes, err := etcdAdapter.List("/desired")
			Ω(err).ShouldNot(HaveOccured())
			Ω(nodes).Should(HaveLen(2))
			Ω(nodes).Should(ContainElement(storeadapter.StoreNode{
				Key:   "/desired/" + app1.AppGuid + "-" + app1.AppVersion,
				Value: app1.DesiredState(0).ToJson(),
				TTL:   conf.DesiredStateTTL - 1,
			}))
			Ω(nodes).Should(ContainElement(storeadapter.StoreNode{
				Key:   "/desired/" + app2.AppGuid + "-" + app2.AppVersion,
				Value: app2.DesiredState(0).ToJson(),
				TTL:   conf.DesiredStateTTL - 1,
			}))
		})
	})

	Describe("Fetching desired state", func() {
		Context("When the desired state is present", func() {
			BeforeEach(func() {
				err := store.SaveDesiredState([]models.DesiredAppState{
					app1.DesiredState(0),
					app2.DesiredState(0),
				})
				Ω(err).ShouldNot(HaveOccured())
			})

			It("can fetch the desired state", func() {
				desired, err := store.GetDesiredState()
				Ω(err).ShouldNot(HaveOccured())
				Ω(desired).Should(HaveLen(2))
				Ω(desired).Should(ContainElement(EqualDesiredState(app1.DesiredState(0))))
				Ω(desired).Should(ContainElement(EqualDesiredState(app2.DesiredState(0))))
			})
		})

		Context("when the desired state is empty", func() {
			BeforeEach(func() {
				err := store.SaveDesiredState([]models.DesiredAppState{
					app1.DesiredState(0),
				})
				Ω(err).ShouldNot(HaveOccured())
				err = store.DeleteDesiredState([]models.DesiredAppState{app1.DesiredState(0)})
				Ω(err).ShouldNot(HaveOccured())
			})

			It("returns an empty array", func() {
				desired, err := store.GetDesiredState()
				Ω(err).ShouldNot(HaveOccured())
				Ω(desired).Should(BeEmpty())
			})
		})

		Context("When the desired state key is missing", func() {
			BeforeEach(func() {
				_, err := etcdAdapter.List("/desired")
				Ω(storeadapter.IsKeyNotFoundError(err)).Should(BeTrue(), "Expected /desired to be missing -- make sure you fully reset the DB before this test.")
			})

			It("returns an empty array and no error", func() {
				desired, err := store.GetDesiredState()
				Ω(err).ShouldNot(HaveOccured())
				Ω(desired).Should(HaveLen(0))
			})
		})
	})

	Describe("Deleting desired state", func() {
		BeforeEach(func() {
			err := store.SaveDesiredState([]models.DesiredAppState{
				app1.DesiredState(0),
				app2.DesiredState(0),
				app3.DesiredState(0),
			})
			Ω(err).ShouldNot(HaveOccured())
		})

		Context("When the desired state is present", func() {
			It("can delete the desired state (and only cares about the relevant fields)", func() {
				toDelete := []models.DesiredAppState{
					models.DesiredAppState{AppGuid: app1.AppGuid, AppVersion: app1.AppVersion},
					models.DesiredAppState{AppGuid: app3.AppGuid, AppVersion: app3.AppVersion},
				}
				err := store.DeleteDesiredState(toDelete)
				Ω(err).ShouldNot(HaveOccured())

				desired, err := store.GetDesiredState()
				Ω(err).ShouldNot(HaveOccured())
				Ω(desired).Should(HaveLen(1))
				Ω(desired).Should(ContainElement(EqualDesiredState(app2.DesiredState(0))))
			})
		})

		Context("When the desired state key is not present", func() {
			It("returns an error, but does leave things in a broken state... for now...", func() {
				toDelete := []models.DesiredAppState{
					models.DesiredAppState{AppGuid: app1.AppGuid, AppVersion: app1.AppVersion},
					models.DesiredAppState{AppGuid: app3.AppGuid, AppVersion: app2.AppVersion}, //oops!
					models.DesiredAppState{AppGuid: app2.AppGuid, AppVersion: app2.AppVersion},
				}
				err := store.DeleteDesiredState(toDelete)
				Ω(storeadapter.IsKeyNotFoundError(err)).Should(BeTrue())

				desired, err := store.GetDesiredState()
				Ω(err).ShouldNot(HaveOccured())
				Ω(desired).Should(HaveLen(2))
				Ω(desired).Should(ContainElement(EqualDesiredState(app2.DesiredState(0))))
				Ω(desired).Should(ContainElement(EqualDesiredState(app3.DesiredState(0))))
			})
		})
	})
})
