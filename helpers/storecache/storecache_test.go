package storecache_test

import (
	"errors"
	. "github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
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

		app1         app.App
		app2         app.App
		app3         app.App
		crashCount   models.CrashCount
		startMessage models.PendingStartMessage
		stopMessage  models.PendingStopMessage
	)

	BeforeEach(func() {
		app1 = app.NewApp()
		app2 = app.NewApp()
		app3 = app.NewApp()

		store = fakestore.NewFakeStore()
		cache = New(store)

		actualState = []models.InstanceHeartbeat{
			app1.InstanceAtIndex(0).Heartbeat(),
			app1.InstanceAtIndex(1).Heartbeat(),
			app1.InstanceAtIndex(2).Heartbeat(),
			app2.InstanceAtIndex(0).Heartbeat(),
		}
		desiredState = []models.DesiredAppState{
			app1.DesiredState(1),
			app3.DesiredState(1),
		}

		store.SaveActualState(actualState)
		store.SaveDesiredState(desiredState)
		store.BumpActualFreshness(time.Unix(10, 0))
		store.BumpDesiredFreshness(time.Unix(10, 0))
		crashCount = models.CrashCount{
			AppGuid:       "abc",
			AppVersion:    "xyz",
			InstanceIndex: 1,
			CrashCount:    12,
		}
		store.SaveCrashCounts([]models.CrashCount{crashCount})

		startMessage = models.NewPendingStartMessage(time.Unix(10, 0), 0, 0, models.Guid(), models.Guid(), 2, 1.0)
		store.SavePendingStartMessages([]models.PendingStartMessage{startMessage})
		stopMessage = models.NewPendingStopMessage(time.Unix(10, 0), 0, 0, models.Guid())
		store.SavePendingStopMessages([]models.PendingStopMessage{stopMessage})
	})

	Describe("Key", func() {
		It("should return the key", func() {
			Ω(cache.Key("abc", "xyz")).Should(Equal("abc-xyz"))
		})
	})

	Describe("Load", func() {
		Context("when all is well", func() {
			BeforeEach(func() {
				err := cache.Load(time.Unix(30, 0))
				Ω(err).ShouldNot(HaveOccured())
			})

			It("loads the actual and desired state", func() {
				Ω(cache.ActualStates).Should(Equal(actualState))
				Ω(cache.DesiredStates).Should(Equal(desiredState))
			})

			It("should build the set of apps", func() {
				Ω(cache.SetOfApps).Should(HaveLen(3))
				Ω(cache.SetOfApps).Should(HaveKey(app1.AppGuid + "-" + app1.AppVersion))
				Ω(cache.SetOfApps).Should(HaveKey(app2.AppGuid + "-" + app2.AppVersion))
				Ω(cache.SetOfApps).Should(HaveKey(app3.AppGuid + "-" + app3.AppVersion))
			})

			It("should index heartbeating instances by app/version guid", func() {
				Ω(cache.HeartbeatingInstancesByApp).Should(HaveLen(2))
				runningApp1 := cache.HeartbeatingInstancesByApp[app1.AppGuid+"-"+app1.AppVersion]
				Ω(runningApp1).Should(HaveLen(3))
				Ω(runningApp1).Should(ContainElement(app1.InstanceAtIndex(0).Heartbeat()))
				Ω(runningApp1).Should(ContainElement(app1.InstanceAtIndex(1).Heartbeat()))
				Ω(runningApp1).Should(ContainElement(app1.InstanceAtIndex(2).Heartbeat()))
				runningApp2 := cache.HeartbeatingInstancesByApp[app2.AppGuid+"-"+app2.AppVersion]
				Ω(runningApp2).Should(HaveLen(1))
				Ω(runningApp2).Should(ContainElement(app2.InstanceAtIndex(0).Heartbeat()))
			})

			It("should index desired state by app/version guid", func() {
				Ω(cache.DesiredByApp).Should(HaveLen(2))
				desiredApp1 := cache.DesiredByApp[app1.AppGuid+"-"+app1.AppVersion]
				Ω(desiredApp1).Should(Equal(app1.DesiredState(1)))
				desiredApp3 := cache.DesiredByApp[app3.AppGuid+"-"+app3.AppVersion]
				Ω(desiredApp3).Should(Equal(app3.DesiredState(1)))
			})

			It("should index heartbeating instances by instance guid", func() {
				Ω(cache.HeartbeatingInstancesByGuid).Should(HaveLen(4))
				instance1 := app1.InstanceAtIndex(0)
				Ω(cache.HeartbeatingInstancesByGuid[instance1.InstanceGuid]).Should(Equal(instance1.Heartbeat()))
				instance2 := app1.InstanceAtIndex(1)
				Ω(cache.HeartbeatingInstancesByGuid[instance2.InstanceGuid]).Should(Equal(instance2.Heartbeat()))
				instance3 := app1.InstanceAtIndex(2)
				Ω(cache.HeartbeatingInstancesByGuid[instance3.InstanceGuid]).Should(Equal(instance3.Heartbeat()))
				instance4 := app2.InstanceAtIndex(0)
				Ω(cache.HeartbeatingInstancesByGuid[instance4.InstanceGuid]).Should(Equal(instance4.Heartbeat()))
			})

			It("should index crash counts by app/version/index", func() {
				Ω(cache.CrashCount(crashCount.AppGuid, crashCount.AppVersion, crashCount.InstanceIndex)).Should(Equal(crashCount))
			})

			It("should return a correctly configured crash count when the crash count is not in the cache", func() {
				crashCount := cache.CrashCount("app-guid", "app-version", 17)
				Ω(crashCount).Should(Equal(models.CrashCount{
					AppGuid:       "app-guid",
					AppVersion:    "app-version",
					InstanceIndex: 17,
					CrashCount:    0,
				}))
			})

			It("should index pending start and stop messages by storekey", func() {
				Ω(cache.PendingStartMessages[startMessage.StoreKey()]).Should(EqualPendingStartMessage(startMessage))
				Ω(cache.PendingStopMessages[stopMessage.StoreKey()]).Should(EqualPendingStopMessage(stopMessage))
			})
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

		Context("when there is an error getting the crash counts", func() {
			It("should return an error", func() {
				store.GetCrashCountsError = errors.New("oops")
				err := cache.Load(time.Unix(30, 0))
				Ω(err).Should(Equal(errors.New("oops")))
			})
		})

		Context("When there is an error getting pending start messages", func() {
			It("should return an error", func() {
				store.GetStartMessagesError = errors.New("oops")
				err := cache.Load(time.Unix(30, 0))
				Ω(err).Should(Equal(errors.New("oops")))
			})
		})

		Context("When there is an error getting pending stop messages", func() {
			It("should return an error", func() {
				store.GetStopMessagesError = errors.New("oops")
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
