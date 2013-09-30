package storecache_test

import (
	"errors"
	. "github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestore"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Storecache", func() {
	var (
		cache *StoreCache
		store *fakestore.FakeStore

		actualState  []models.InstanceHeartbeat
		desiredState []models.DesiredAppState

		app1 app.App
		app2 app.App
		app3 app.App
	)

	BeforeEach(func() {
		app1 = app.NewApp()
		app2 = app.NewApp()
		app3 = app.NewApp()

		store = fakestore.NewFakeStore()
		cache = New(store)

		actualState = []models.InstanceHeartbeat{
			app1.GetInstance(0).Heartbeat(0),
			app1.GetInstance(1).Heartbeat(0),
			app1.GetInstance(2).Heartbeat(0),
			app2.GetInstance(0).Heartbeat(0),
		}
		desiredState = []models.DesiredAppState{
			app1.DesiredState(0),
			app3.DesiredState(0),
		}

		store.SaveActualState(actualState)
		store.SaveDesiredState(desiredState)
		store.BumpActualFreshness(time.Unix(10, 0))
		store.BumpDesiredFreshness(time.Unix(10, 0))
	})

	Describe("Key", func() {
		It("should return the key", func() {
			Ω(cache.Key("abc", "xyz")).Should(Equal("abc-xyz"))
		})
	})

	Describe("Load", func() {
		It("should not return an error", func() {
			err := cache.Load(time.Unix(30, 0))
			Ω(err).ShouldNot(HaveOccured())
		})

		It("loads", func() {
			cache.Load(time.Unix(30, 0))
			Ω(cache.ActualStates).Should(Equal(actualState))
			Ω(cache.DesiredStates).Should(Equal(desiredState))

			Ω(cache.SetOfApps).Should(HaveLen(3))
			Ω(cache.SetOfApps).Should(HaveKey(app1.AppGuid + "-" + app1.AppVersion))
			Ω(cache.SetOfApps).Should(HaveKey(app2.AppGuid + "-" + app2.AppVersion))
			Ω(cache.SetOfApps).Should(HaveKey(app3.AppGuid + "-" + app3.AppVersion))

			Ω(cache.RunningByApp).Should(HaveLen(2))
			runningApp1 := cache.RunningByApp[app1.AppGuid+"-"+app1.AppVersion]
			Ω(runningApp1).Should(HaveLen(3))
			Ω(runningApp1).Should(ContainElement(app1.GetInstance(0).Heartbeat(0)))
			Ω(runningApp1).Should(ContainElement(app1.GetInstance(1).Heartbeat(0)))
			Ω(runningApp1).Should(ContainElement(app1.GetInstance(2).Heartbeat(0)))
			runningApp2 := cache.RunningByApp[app2.AppGuid+"-"+app2.AppVersion]
			Ω(runningApp2).Should(HaveLen(1))
			Ω(runningApp2).Should(ContainElement(app2.GetInstance(0).Heartbeat(0)))

			Ω(cache.DesiredByApp).Should(HaveLen(2))
			desiredApp1 := cache.DesiredByApp[app1.AppGuid+"-"+app1.AppVersion]
			Ω(desiredApp1).Should(Equal(app1.DesiredState(0)))
			desiredApp3 := cache.DesiredByApp[app3.AppGuid+"-"+app3.AppVersion]
			Ω(desiredApp3).Should(Equal(app3.DesiredState(0)))

			Ω(cache.RunningByInstance).Should(HaveLen(4))
			instance1 := app1.GetInstance(0)
			Ω(cache.RunningByInstance[instance1.InstanceGuid]).Should(Equal(instance1.Heartbeat(0)))
			instance2 := app1.GetInstance(1)
			Ω(cache.RunningByInstance[instance2.InstanceGuid]).Should(Equal(instance2.Heartbeat(0)))
			instance3 := app1.GetInstance(2)
			Ω(cache.RunningByInstance[instance3.InstanceGuid]).Should(Equal(instance3.Heartbeat(0)))
			instance4 := app2.GetInstance(0)
			Ω(cache.RunningByInstance[instance4.InstanceGuid]).Should(Equal(instance4.Heartbeat(0)))
		})

		Context("when there is an error getting desired state", func() {
			It("should return an error", func() {
				store.GetDesiredStateError = errors.New("oops")
				err := cache.Load(time.Unix(30, 0))
				Ω(err).Should(Equal(errors.New("oops")))
			})
		})

		Context("when there is an error getting actual state", func() {
			It("should return an error", func() {
				store.GetActualStateError = errors.New("oops")
				err := cache.Load(time.Unix(30, 0))
				Ω(err).Should(Equal(errors.New("oops")))
			})
		})

		Context("when the desired state is not fresh", func() {
			BeforeEach(func() {
				store.DesiredFreshnessTimestamp = time.Time{}
				store.BumpActualFreshness(time.Unix(10, 0))
			})

			It("should return an error", func() {
				err := cache.Load(time.Unix(30, 0))
				Ω(err.Error()).Should(Equal("Desired state is not fresh"))
				Ω(cache.ActualStates).Should(BeEmpty())
				Ω(cache.DesiredStates).Should(BeEmpty())
			})
		})

		Context("when the actual state is not fresh", func() {
			BeforeEach(func() {
				store.ActualFreshnessTimestamp = time.Time{}
				store.BumpDesiredFreshness(time.Unix(10, 0))
			})

			It("should pass in the correct timestamp to the actual state", func() {
				cache.Load(time.Unix(30, 0))
				Ω(store.ActualFreshnessComparisonTimestamp).Should(Equal(time.Unix(30, 0)))
			})

			It("should not send any start or stop messages", func() {
				err := cache.Load(time.Unix(30, 0))
				Ω(err.Error()).Should(Equal("Actual state is not fresh"))
				Ω(cache.ActualStates).Should(BeEmpty())
				Ω(cache.DesiredStates).Should(BeEmpty())
			})
		})
	})
})
