package storecache_test

import (
	"errors"
	. "github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
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

		fixture1     appfixture.AppFixture
		fixture2     appfixture.AppFixture
		fixture3     appfixture.AppFixture
		crashCount   models.CrashCount
		startMessage models.PendingStartMessage
		stopMessage  models.PendingStopMessage
	)

	BeforeEach(func() {
		fixture1 = appfixture.NewAppFixture()
		fixture2 = appfixture.NewAppFixture()
		fixture3 = appfixture.NewAppFixture()

		store = fakestore.NewFakeStore()
		cache = New(store)

		actualState = []models.InstanceHeartbeat{
			fixture1.InstanceAtIndex(0).Heartbeat(),
			fixture1.InstanceAtIndex(1).Heartbeat(),
			fixture1.InstanceAtIndex(2).Heartbeat(),
			fixture2.InstanceAtIndex(0).Heartbeat(),
		}
		desiredState = []models.DesiredAppState{
			fixture1.DesiredState(1),
			fixture3.DesiredState(1),
		}

		store.SaveActualState(actualState...)
		store.SaveDesiredState(desiredState...)
		store.BumpActualFreshness(time.Unix(10, 0))
		store.BumpDesiredFreshness(time.Unix(10, 0))
		crashCount = models.CrashCount{
			AppGuid:       fixture1.AppGuid,
			AppVersion:    fixture1.AppVersion,
			InstanceIndex: 1,
			CrashCount:    12,
		}
		store.SaveCrashCounts(crashCount)

		startMessage = models.NewPendingStartMessage(time.Unix(10, 0), 0, 0, models.Guid(), models.Guid(), 2, 1.0)
		store.SavePendingStartMessages(startMessage)
		stopMessage = models.NewPendingStopMessage(time.Unix(10, 0), 0, 0, models.Guid(), models.Guid(), models.Guid())
		store.SavePendingStopMessages(stopMessage)
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
				Ω(cache.ActualStates).Should(HaveLen(4))
				for _, actual := range actualState {
					Ω(cache.ActualStates).Should(ContainElement(actual))
				}

				Ω(cache.DesiredStates).Should(HaveLen(2))
				for _, desired := range desiredState {
					Ω(cache.DesiredStates).Should(ContainElement(desired))
				}
			})

			It("should build the set of apps", func() {
				Ω(cache.Apps).Should(HaveLen(3))
				Ω(cache.Apps).Should(HaveKey(fixture1.AppGuid + "-" + fixture1.AppVersion))
				Ω(cache.Apps).Should(HaveKey(fixture2.AppGuid + "-" + fixture2.AppVersion))
				Ω(cache.Apps).Should(HaveKey(fixture3.AppGuid + "-" + fixture3.AppVersion))

				a1 := cache.Apps[fixture1.AppGuid+"-"+fixture1.AppVersion]
				Ω(a1.Desired).Should(EqualDesiredState(fixture1.DesiredState(1)))
				Ω(a1.InstanceHeartbeats).Should(HaveLen(3))
				Ω(a1.InstanceHeartbeats).Should(ContainElement(fixture1.InstanceAtIndex(0).Heartbeat()))
				Ω(a1.InstanceHeartbeats).Should(ContainElement(fixture1.InstanceAtIndex(1).Heartbeat()))
				Ω(a1.InstanceHeartbeats).Should(ContainElement(fixture1.InstanceAtIndex(2).Heartbeat()))
				Ω(a1.CrashCounts[1]).Should(Equal(crashCount))

				a2 := cache.Apps[fixture2.AppGuid+"-"+fixture2.AppVersion]
				Ω(a2.Desired).Should(BeZero())
				Ω(a2.InstanceHeartbeats).Should(HaveLen(1))
				Ω(a2.InstanceHeartbeats).Should(ContainElement(fixture2.InstanceAtIndex(0).Heartbeat()))
				Ω(a2.CrashCounts).Should(BeEmpty())

				a3 := cache.Apps[fixture3.AppGuid+"-"+fixture3.AppVersion]
				Ω(a3.Desired).Should(EqualDesiredState(fixture3.DesiredState(1)))
				Ω(a3.InstanceHeartbeats).Should(HaveLen(0))
				Ω(a3.CrashCounts).Should(BeEmpty())
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
				Ω(err).Should(Equal(cache.DesiredIsNotFreshError))
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
				Ω(err).Should(Equal(cache.ActualIsNotFreshError))
				Ω(cache.ActualStates).Should(BeEmpty())
				Ω(cache.DesiredStates).Should(BeEmpty())
			})
		})
	})
})
