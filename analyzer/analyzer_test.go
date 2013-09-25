package analyzer_test

import (
	. "github.com/cloudfoundry/hm9000/analyzer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
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
		return models.NewQueueStartMessage(timeProvider.Time(), conf.GracePeriod, 0, a.AppGuid, a.AppVersion, indexToStart)
	}

	newStopMessage := func(instance app.Instance) models.QueueStopMessage {
		return models.NewQueueStopMessage(timeProvider.Time(), 0, conf.GracePeriod, instance.InstanceGuid)
	}

	assertStartMessages := func(messages ...models.QueueStartMessage) {
		Ω(outbox.StartMessages).Should(HaveLen(len(messages)))
		for _, message := range messages {
			Ω(outbox.StartMessages[message.StoreKey()]).Should(Equal(message))
		}
	}

	assertStopMessages := func(messages ...models.QueueStopMessage) {
		Ω(outbox.StopMessages).Should(HaveLen(len(messages)))
		for _, message := range messages {
			Ω(outbox.StopMessages[message.StoreKey()]).Should(Equal(message))
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

		analyzer = New(store, outbox, timeProvider, conf)
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

		Context("When there are stopped apps and no running instances for that app", func() {
			BeforeEach(func() {
				desired := a.DesiredState(10)
				desired.State = models.AppStateStopped
				desired.NumberOfInstances = 3
				store.SaveDesiredState([]models.DesiredAppState{
					desired,
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

			Context("and the desired app is in the STOPPED state", func() {
				BeforeEach(func() {
					desired := a.DesiredState(0)
					desired.NumberOfInstances = 3
					desired.State = models.AppStateStopped
					store.SaveDesiredState([]models.DesiredAppState{
						desired,
					})
				})

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

	Describe("Interesting edge cases involving index-mismatches", func() {
		BeforeEach(func() {
			desired := a.DesiredState(0)
			desired.NumberOfInstances = 3
			store.SaveDesiredState([]models.DesiredAppState{
				desired,
			})
		})

		Context("when *enough* apps are running, but there are an indices missing", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(3).Heartbeat(0),
					a.GetInstance(4).Heartbeat(0),
				})
			})

			It("should return a start message containing the missing indices and *no* stop message", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StopMessages).Should(BeEmpty())
				assertStartMessages(newStartMessage(a, 0), newStartMessage(a, 2))
			})
		})

		Context("when more than *enough* apps are running, but there are indices missing", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(3).Heartbeat(0),
					a.GetInstance(4).Heartbeat(0),
					a.GetInstance(5).Heartbeat(0),
					a.GetInstance(6).Heartbeat(0),
				})
			})

			It("should return a start message containing the missing indices and a stop message for the extra instances", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				assertStartMessages(newStartMessage(a, 0), newStartMessage(a, 2))
				assertStopMessages(newStopMessage(a.GetInstance(5)), newStopMessage(a.GetInstance(6)))
			})
		})

		Context("when the missing indices start", func() {
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

	Describe("Processing multiple apps", func() {
		var (
			otherApp      app.App
			yetAnotherApp app.App
			olderApp      app.App
		)

		BeforeEach(func() {
			otherApp = app.NewApp()
			olderApp = app.NewApp()
			yetAnotherApp = app.NewApp()
			olderApp.AppGuid = a.AppGuid

			olderDesired := olderApp.DesiredState(0)
			olderDesired.State = models.AppStateStopped

			otherDesired := otherApp.DesiredState(0)
			otherDesired.NumberOfInstances = 3

			yetAnotherDesired := yetAnotherApp.DesiredState(0)
			yetAnotherDesired.NumberOfInstances = 2

			store.SaveDesiredState([]models.DesiredAppState{
				a.DesiredState(0),
				otherDesired,
				olderDesired,
				yetAnotherDesired,
			})
			store.SaveActualState([]models.InstanceHeartbeat{
				a.GetInstance(0).Heartbeat(0),
				a.GetInstance(1).Heartbeat(0),
				olderApp.GetInstance(0).Heartbeat(0),
				otherApp.GetInstance(0).Heartbeat(0),
				otherApp.GetInstance(2).Heartbeat(0),
			})
		})

		It("should analyze each app-version combination separately", func() {
			err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
			assertStartMessages(newStartMessage(otherApp, 1), newStartMessage(yetAnotherApp, 0), newStartMessage(yetAnotherApp, 1))
			assertStopMessages(newStopMessage(a.GetInstance(1)), newStopMessage(olderApp.GetInstance(0)))
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
