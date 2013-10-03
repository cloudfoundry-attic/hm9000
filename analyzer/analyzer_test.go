package analyzer_test

import (
	. "github.com/cloudfoundry/hm9000/analyzer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/fakeoutbox"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestore"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"

	"errors"
	"time"
)

var _ = Describe("Analyzer", func() {
	var (
		analyzer     *Analyzer
		store        *fakestore.FakeStore
		outbox       *fakeoutbox.FakeOutbox
		timeProvider *faketimeprovider.FakeTimeProvider
		a            app.App
	)

	conf, _ := config.DefaultConfig()

	newStartMessage := func(a app.App, indexToStart int) models.QueueStartMessage {
		return models.NewQueueStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, indexToStart)
	}

	newStopMessage := func(instance app.Instance) models.QueueStopMessage {
		return models.NewQueueStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), instance.InstanceGuid)
	}

	assertStartMessages := func(messages ...models.QueueStartMessage) {
		Ω(outbox.StartMessages).Should(HaveLen(len(messages)))
		for _, message := range messages {
			Ω(outbox.StartMessages[message.StoreKey()]).ShouldNot(BeZero())
			candidateMatch := outbox.StartMessages[message.StoreKey()]
			candidateMatch.MessageId = message.MessageId
			Ω(candidateMatch).Should(Equal(message))
		}
	}

	assertStopMessages := func(messages ...models.QueueStopMessage) {
		Ω(outbox.StopMessages).Should(HaveLen(len(messages)))
		for _, message := range messages {
			Ω(outbox.StopMessages[message.StoreKey()]).ShouldNot(BeZero())
			candidateMatch := outbox.StopMessages[message.StoreKey()]
			candidateMatch.MessageId = message.MessageId
			Ω(candidateMatch).Should(Equal(message))
		}
	}

	BeforeEach(func() {
		store = fakestore.NewFakeStore()
		outbox = fakeoutbox.NewFakeOutbox()
		timeProvider = &faketimeprovider.FakeTimeProvider{}
		timeProvider.TimeToProvide = time.Unix(1000, 0)

		a = app.NewApp()

		store.BumpActualFreshness(time.Unix(100, 0))
		store.BumpDesiredFreshness(time.Unix(100, 0))

		analyzer = New(store, outbox, timeProvider, fakelogger.NewFakeLogger(), conf)
	})

	Describe("Handling store errors", func() {
		Context("When fetching desired state fails with an error", func() {
			BeforeEach(func() {
				store.GetDesiredStateError = errors.New("oops!")
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).Should(Equal(errors.New("oops!")))
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})

		Context("When fetching actual state fails with an error", func() {
			BeforeEach(func() {
				store.GetActualStateError = errors.New("oops!")
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).Should(Equal(errors.New("oops!")))
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})
	})

	Describe("The steady state", func() {
		Context("When there are no desired or running apps", func() {
			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})

		Context("When the desired number of instances and the running number of instances match", func() {
			BeforeEach(func() {
				desired := a.DesiredState(0)
				desired.State = models.AppStateStarted
				desired.NumberOfInstances = 3
				store.SaveDesiredState([]models.DesiredAppState{
					desired,
				})
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(0).Heartbeat(0),
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(2).Heartbeat(0),
				})
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})
	})

	Describe("Starting missing instances", func() {
		Context("where an app has desired instances", func() {
			BeforeEach(func() {
				desired := a.DesiredState(0)
				desired.NumberOfInstances = 4
				store.SaveDesiredState([]models.DesiredAppState{
					desired,
				})
			})

			Context("and none of the instances are running", func() {
				It("should send a start message for each of the missing instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.StopMessages).Should(BeEmpty())
					assertStartMessages(newStartMessage(a, 0), newStartMessage(a, 1), newStartMessage(a, 2), newStartMessage(a, 3))
				})
			})

			Context("but only some of the instances are running", func() {
				BeforeEach(func() {
					store.SaveActualState([]models.InstanceHeartbeat{
						a.GetInstance(0).Heartbeat(0),
						a.GetInstance(2).Heartbeat(0),
					})
				})

				It("should return a start message containing only the missing indices", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.StopMessages).Should(BeEmpty())
					assertStartMessages(newStartMessage(a, 1), newStartMessage(a, 3))
				})
			})
		})
	})

	Describe("Stopping extra instances", func() {
		Context("When there are running instances", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(0).Heartbeat(0),
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(2).Heartbeat(0),
				})
			})

			Context("but no desired instances", func() {
				It("should return an array of stop messages for the extra instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.StartMessages).Should(BeEmpty())
					assertStopMessages(newStopMessage(a.GetInstance(0)), newStopMessage(a.GetInstance(1)), newStopMessage(a.GetInstance(2)))
				})
			})

			Context("and the desired app desires fewer instances", func() {
				BeforeEach(func() {
					desired := a.DesiredState(0)
					desired.NumberOfInstances = 1
					store.SaveDesiredState([]models.DesiredAppState{
						desired,
					})
				})

				It("should return an array of stop messages for the (correct) extra instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.StartMessages).Should(BeEmpty())
					assertStopMessages(newStopMessage(a.GetInstance(1)), newStopMessage(a.GetInstance(2)))
				})
			})

		})
	})

	Describe("Interesting edge cases involving extra instances (instances at indices >= numdesired)", func() {
		BeforeEach(func() {
			desired := a.DesiredState(0)
			desired.NumberOfInstances = 3
			store.SaveDesiredState([]models.DesiredAppState{
				desired,
			})
		})
		Context("when there are indices missing", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(3).Heartbeat(0),
					a.GetInstance(4).Heartbeat(0),
					a.GetInstance(5).Heartbeat(0),
					a.GetInstance(6).Heartbeat(0),
				})
			})

			It("should return a start message containing the missing indices and no stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				assertStartMessages(newStartMessage(a, 0), newStartMessage(a, 2))
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})

		Context("when all desired indices are present", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(0).Heartbeat(0),
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(2).Heartbeat(0),
					a.GetInstance(3).Heartbeat(0),
					a.GetInstance(4).Heartbeat(0),
				})
			})

			It("should stop the extra indices", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StartMessages).Should(BeEmpty())
				assertStopMessages(newStopMessage(a.GetInstance(3)), newStopMessage(a.GetInstance(4)))
			})
		})
	})

	Context("When multiple instances report on the same index", func() {
		var (
			duplicateInstance1 app.Instance
			duplicateInstance2 app.Instance
			duplicateInstance3 app.Instance
		)

		BeforeEach(func() {
			desired := a.DesiredState(0)
			desired.NumberOfInstances = 3
			store.SaveDesiredState([]models.DesiredAppState{
				desired,
			})

			duplicateInstance1 = a.GetInstance(2)
			duplicateInstance1.InstanceGuid = models.Guid()
			duplicateInstance2 = a.GetInstance(2)
			duplicateInstance2.InstanceGuid = models.Guid()
			duplicateInstance3 = a.GetInstance(2)
			duplicateInstance3.InstanceGuid = models.Guid()
		})

		Context("When there are missing instances on other indices", func() {
			It("should not schedule any stops and start the missing indices", func() {
				//[-,-,2|2|2|2]
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(2).Heartbeat(0),
					duplicateInstance1.Heartbeat(0),
					duplicateInstance2.Heartbeat(0),
					duplicateInstance3.Heartbeat(0),
				})

				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StopMessages).Should(BeEmpty())
				assertStartMessages(newStartMessage(a, 0), newStartMessage(a, 1))
			})
		})

		Context("When all the other indices has instances", func() {
			It("should schedule a stop for every instance at the duplicated index with increasing delays", func() {
				//[0,1,2|2|2] < stop 2,2,2 with increasing delays etc...
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(0).Heartbeat(0),
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(2).Heartbeat(0),
					duplicateInstance1.Heartbeat(0),
					duplicateInstance2.Heartbeat(0),
				})

				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StartMessages).Should(BeEmpty())
				stop0 := newStopMessage(a.GetInstance(2))
				stop0.SendOn = stop0.SendOn + int64(conf.GracePeriod())
				stop1 := newStopMessage(duplicateInstance1)
				stop1.SendOn = stop1.SendOn + int64(conf.GracePeriod()*2)
				stop2 := newStopMessage(duplicateInstance2)
				stop2.SendOn = stop2.SendOn + int64(conf.GracePeriod()*3)
				assertStopMessages(stop0, stop1, stop2)
			})
		})

		Context("When the duplicated index is also an unwanted index", func() {
			var (
				duplicateExtraInstance1 app.Instance
				duplicateExtraInstance2 app.Instance
			)

			BeforeEach(func() {
				duplicateExtraInstance1 = a.GetInstance(3)
				duplicateExtraInstance1.InstanceGuid = models.Guid()
				duplicateExtraInstance2 = a.GetInstance(3)
				duplicateExtraInstance2.InstanceGuid = models.Guid()
			})

			It("should terminate the extra indices with extreme prejudice", func() {
				//[0,1,2,3,3,3] < stop 3,3,3
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(0).Heartbeat(0),
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(2).Heartbeat(0),
					a.GetInstance(3).Heartbeat(0),
					duplicateExtraInstance1.Heartbeat(0),
					duplicateExtraInstance2.Heartbeat(0),
				})

				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StartMessages).Should(BeEmpty())
				stop0 := newStopMessage(a.GetInstance(3))
				stop1 := newStopMessage(duplicateExtraInstance1)
				stop2 := newStopMessage(duplicateExtraInstance2)
				assertStopMessages(stop0, stop1, stop2)
			})
		})
	})

	Describe("Processing multiple apps", func() {
		var (
			otherApp      app.App
			yetAnotherApp app.App
			undesiredApp  app.App
		)

		BeforeEach(func() {
			otherApp = app.NewApp()
			undesiredApp = app.NewApp()
			yetAnotherApp = app.NewApp()
			undesiredApp.AppGuid = a.AppGuid

			otherDesired := otherApp.DesiredState(0)
			otherDesired.NumberOfInstances = 3

			yetAnotherDesired := yetAnotherApp.DesiredState(0)
			yetAnotherDesired.NumberOfInstances = 2

			store.SaveDesiredState([]models.DesiredAppState{
				a.DesiredState(0),
				otherDesired,
				yetAnotherDesired,
			})
			store.SaveActualState([]models.InstanceHeartbeat{
				a.GetInstance(0).Heartbeat(0),
				a.GetInstance(1).Heartbeat(0),
				undesiredApp.GetInstance(0).Heartbeat(0),
				otherApp.GetInstance(0).Heartbeat(0),
				otherApp.GetInstance(2).Heartbeat(0),
			})
		})

		It("should analyze each app-version combination separately", func() {
			err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
			assertStartMessages(newStartMessage(otherApp, 1), newStartMessage(yetAnotherApp, 0), newStartMessage(yetAnotherApp, 1))
			assertStopMessages(newStopMessage(a.GetInstance(1)), newStopMessage(undesiredApp.GetInstance(0)))
		})
	})

	Context("When the store is not fresh", func() {
		BeforeEach(func() {
			store.Reset()

			desired := a.DesiredState(0)
			//this setup would, ordinarily, trigger a start and a stop
			store.SaveDesiredState([]models.DesiredAppState{
				desired,
			})
			store.SaveActualState([]models.InstanceHeartbeat{
				app.NewApp().GetInstance(0).Heartbeat(0),
			})
		})

		Context("when the desired state is not fresh", func() {
			BeforeEach(func() {
				store.BumpActualFreshness(time.Unix(10, 0))
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err.Error()).Should(Equal("Desired state is not fresh"))
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})

		Context("when the desired state fails to fetch", func() {
			BeforeEach(func() {
				store.BumpActualFreshness(time.Unix(10, 0))
				store.BumpDesiredFreshness(time.Unix(10, 0))
				store.IsDesiredStateFreshError = errors.New("foo")
			})

			It("should return the store's error and not send any start/stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).Should(Equal(store.IsDesiredStateFreshError))
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})

		Context("when the actual state is not fresh", func() {
			BeforeEach(func() {
				store.BumpDesiredFreshness(time.Unix(10, 0))
			})

			It("should pass in the correct timestamp to the actual state", func() {
				analyzer.Analyze()
				Ω(store.ActualFreshnessComparisonTimestamp).Should(Equal(timeProvider.TimeToProvide))
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err.Error()).Should(Equal("Actual state is not fresh"))
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})

		Context("when the actual state fails to fetch", func() {
			BeforeEach(func() {
				store.BumpActualFreshness(time.Unix(10, 0))
				store.BumpDesiredFreshness(time.Unix(10, 0))
				store.IsActualStateFreshError = errors.New("foo")
			})

			It("should return the store's error and not send any start/stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).Should(Equal(store.IsActualStateFreshError))
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})
	})
})
