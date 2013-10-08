package analyzer_test

import (
	. "github.com/cloudfoundry/hm9000/analyzer"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
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
				Ω(outbox.PendingStartMessages).Should(BeEmpty())
				Ω(outbox.PendingStopMessages).Should(BeEmpty())
			})
		})

		Context("When fetching actual state fails with an error", func() {
			BeforeEach(func() {
				store.GetActualStateError = errors.New("oops!")
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).Should(Equal(errors.New("oops!")))
				Ω(outbox.PendingStartMessages).Should(BeEmpty())
				Ω(outbox.PendingStopMessages).Should(BeEmpty())
			})
		})
	})

	Describe("The steady state", func() {
		Context("When there are no desired or running apps", func() {
			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.PendingStartMessages).Should(BeEmpty())
				Ω(outbox.PendingStopMessages).Should(BeEmpty())
			})
		})

		Context("When the desired number of instances and the running number of instances match", func() {
			BeforeEach(func() {
				desired := a.DesiredState()
				desired.State = models.AppStateStarted
				desired.NumberOfInstances = 3
				store.SaveDesiredState([]models.DesiredAppState{
					desired,
				})
				store.SaveActualState([]models.InstanceHeartbeat{
					a.InstanceAtIndex(0).Heartbeat(),
					a.InstanceAtIndex(1).Heartbeat(),
					a.InstanceAtIndex(2).Heartbeat(),
				})
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.PendingStartMessages).Should(BeEmpty())
				Ω(outbox.PendingStopMessages).Should(BeEmpty())
			})
		})
	})

	Describe("Starting missing instances", func() {
		Context("where an app has desired instances", func() {
			BeforeEach(func() {
				desired := a.DesiredState()
				desired.NumberOfInstances = 2
				store.SaveDesiredState([]models.DesiredAppState{
					desired,
				})
			})

			Context("and none of the instances are running", func() {
				It("should send a start message for each of the missing instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.PendingStopMessages).Should(BeEmpty())
					Ω(outbox.PendingStartMessages).Should(HaveLen(2))

					expectedMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 0, 1)
					Ω(outbox.PendingStartMessages).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))

					expectedMessage = models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 1, 1)
					Ω(outbox.PendingStartMessages).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))
				})

				It("should set the priority to 1", func() {
					analyzer.Analyze()
					for _, message := range outbox.PendingStartMessages {
						Ω(message.Priority).Should(Equal(1.0))
					}
				})
			})

			Context("but only some of the instances are running", func() {
				BeforeEach(func() {
					store.SaveActualState([]models.InstanceHeartbeat{
						a.InstanceAtIndex(0).Heartbeat(),
					})
				})

				It("should return a start message containing only the missing indices", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.PendingStopMessages).Should(BeEmpty())

					Ω(outbox.PendingStartMessages).Should(HaveLen(1))

					expectedMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 1, 0.5)
					Ω(outbox.PendingStartMessages).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))
				})

				It("should set the priority to 0.5", func() {
					analyzer.Analyze()
					for _, message := range outbox.PendingStartMessages {
						Ω(message.Priority).Should(Equal(0.5))
					}
				})
			})
		})
	})

	Describe("Stopping extra instances", func() {
		Context("When there are running instances", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.InstanceAtIndex(0).Heartbeat(),
					a.InstanceAtIndex(1).Heartbeat(),
					a.InstanceAtIndex(2).Heartbeat(),
				})
			})

			Context("but no desired instances", func() {
				It("should return an array of stop messages for the extra instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.PendingStartMessages).Should(BeEmpty())
					Ω(outbox.PendingStopMessages).Should(HaveLen(3))

					expectedMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(0).InstanceGuid)
					Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

					expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(1).InstanceGuid)
					Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

					expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(2).InstanceGuid)
					Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))
				})
			})

			Context("and the desired app desires fewer instances", func() {
				BeforeEach(func() {
					desired := a.DesiredState()
					desired.NumberOfInstances = 1
					store.SaveDesiredState([]models.DesiredAppState{
						desired,
					})
				})

				It("should return an array of stop messages for the (correct) extra instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.PendingStartMessages).Should(BeEmpty())

					Ω(outbox.PendingStopMessages).Should(HaveLen(2))

					expectedMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(1).InstanceGuid)
					Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

					expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(2).InstanceGuid)
					Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))
				})
			})

		})
	})

	Describe("Handling crashed instances", func() {
		JustBeforeEach(func() {
			err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
		})

		Context("When there are multiple crashed instances on the same index", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.CrashedInstanceHeartbeatAtIndex(0),
					a.CrashedInstanceHeartbeatAtIndex(0),
				})
			})

			Context("when the app is desired", func() {
				BeforeEach(func() {
					store.SaveDesiredState([]models.DesiredAppState{
						a.DesiredState(),
					})
				})

				It("should not try to stop crashed instances", func() {
					Ω(outbox.PendingStopMessages).Should(BeEmpty())
				})

				It("should try to start the instance immediately", func() {
					Ω(outbox.PendingStartMessages).Should(HaveLen(1))
					expectMessage := models.NewPendingStartMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.AppGuid, a.AppVersion, 0, 1.0)
					Ω(outbox.PendingStartMessages).Should(ContainElement(EqualPendingStartMessage(expectMessage)))
				})

				Context("when there is a running instance on the same index", func() {
					BeforeEach(func() {
						store.SaveActualState([]models.InstanceHeartbeat{a.InstanceAtIndex(0).Heartbeat()})
					})

					It("should not try to stop the running instance!", func() {
						Ω(outbox.PendingStopMessages).Should(BeEmpty())
					})

					It("should not try to start the instance", func() {
						Ω(outbox.PendingStartMessages).Should(BeEmpty())
					})
				})
			})

			Context("when the app is not desired", func() {
				It("should not try to stop crashed instances", func() {
					Ω(outbox.PendingStopMessages).Should(BeEmpty())
				})

				It("should not try to start the instance", func() {
					Ω(outbox.PendingStartMessages).Should(BeEmpty())
				})
			})
		})
	})

	Describe("Interesting edge cases involving extra instances (instances at indices >= numdesired)", func() {
		BeforeEach(func() {
			desired := a.DesiredState()
			desired.NumberOfInstances = 3
			store.SaveDesiredState([]models.DesiredAppState{
				desired,
			})
		})
		Context("when there are indices missing", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.InstanceAtIndex(1).Heartbeat(),
					a.InstanceAtIndex(3).Heartbeat(),
					a.InstanceAtIndex(4).Heartbeat(),
					a.InstanceAtIndex(5).Heartbeat(),
					a.InstanceAtIndex(6).Heartbeat(),
				})
			})

			It("should return a start message containing the missing indices and no stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())

				Ω(outbox.PendingStartMessages).Should(HaveLen(2))

				expectedMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 0, 2.0/3.0)
				Ω(outbox.PendingStartMessages).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))

				expectedMessage = models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 2, 2.0/3.0)
				Ω(outbox.PendingStartMessages).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))

				Ω(outbox.PendingStopMessages).Should(BeEmpty())
			})
		})

		Context("when all desired indices are present", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.InstanceAtIndex(0).Heartbeat(),
					a.InstanceAtIndex(1).Heartbeat(),
					a.InstanceAtIndex(2).Heartbeat(),
					a.InstanceAtIndex(3).Heartbeat(),
					a.InstanceAtIndex(4).Heartbeat(),
				})
			})

			It("should stop the extra indices", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.PendingStartMessages).Should(BeEmpty())

				Ω(outbox.PendingStopMessages).Should(HaveLen(2))

				expectedMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(3).InstanceGuid)
				Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(4).InstanceGuid)
				Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))
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
			desired := a.DesiredState()
			desired.NumberOfInstances = 3
			store.SaveDesiredState([]models.DesiredAppState{
				desired,
			})

			duplicateInstance1 = a.InstanceAtIndex(2)
			duplicateInstance1.InstanceGuid = models.Guid()
			duplicateInstance2 = a.InstanceAtIndex(2)
			duplicateInstance2.InstanceGuid = models.Guid()
			duplicateInstance3 = a.InstanceAtIndex(2)
			duplicateInstance3.InstanceGuid = models.Guid()
		})

		Context("When there are missing instances on other indices", func() {
			It("should not schedule any stops and start the missing indices", func() {
				//[-,-,2|2|2|2]
				store.SaveActualState([]models.InstanceHeartbeat{
					a.InstanceAtIndex(2).Heartbeat(),
					duplicateInstance1.Heartbeat(),
					duplicateInstance2.Heartbeat(),
					duplicateInstance3.Heartbeat(),
				})

				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.PendingStopMessages).Should(BeEmpty())

				Ω(outbox.PendingStartMessages).Should(HaveLen(2))

				expectedMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 0, 2.0/3.0)
				Ω(outbox.PendingStartMessages).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))

				expectedMessage = models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 1, 2.0/3.0)
				Ω(outbox.PendingStartMessages).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))
			})
		})

		Context("When all the other indices has instances", func() {
			It("should schedule a stop for every running instance at the duplicated index with increasing delays", func() {
				//[0,1,2|2|2] < stop 2,2,2 with increasing delays etc...
				crashedHeartbeat := duplicateInstance3.Heartbeat()
				crashedHeartbeat.State = models.InstanceStateCrashed
				store.SaveActualState([]models.InstanceHeartbeat{
					a.InstanceAtIndex(0).Heartbeat(),
					a.InstanceAtIndex(1).Heartbeat(),
					a.InstanceAtIndex(2).Heartbeat(),
					duplicateInstance1.Heartbeat(),
					duplicateInstance2.Heartbeat(),
					crashedHeartbeat,
				})

				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.PendingStartMessages).Should(BeEmpty())

				Ω(outbox.PendingStopMessages).Should(HaveLen(3))

				expectedMessage := models.NewPendingStopMessage(timeProvider.Time(), conf.GracePeriod(), conf.GracePeriod(), a.InstanceAtIndex(2).InstanceGuid)
				Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), conf.GracePeriod()*2, conf.GracePeriod(), duplicateInstance1.InstanceGuid)
				Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), conf.GracePeriod()*3, conf.GracePeriod(), duplicateInstance2.InstanceGuid)
				Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))
			})
		})

		Context("When the duplicated index is also an unwanted index", func() {
			var (
				duplicateExtraInstance1 app.Instance
				duplicateExtraInstance2 app.Instance
			)

			BeforeEach(func() {
				duplicateExtraInstance1 = a.InstanceAtIndex(3)
				duplicateExtraInstance1.InstanceGuid = models.Guid()
				duplicateExtraInstance2 = a.InstanceAtIndex(3)
				duplicateExtraInstance2.InstanceGuid = models.Guid()
			})

			It("should terminate the extra indices with extreme prejudice", func() {
				//[0,1,2,3,3,3] < stop 3,3,3
				store.SaveActualState([]models.InstanceHeartbeat{
					a.InstanceAtIndex(0).Heartbeat(),
					a.InstanceAtIndex(1).Heartbeat(),
					a.InstanceAtIndex(2).Heartbeat(),
					a.InstanceAtIndex(3).Heartbeat(),
					duplicateExtraInstance1.Heartbeat(),
					duplicateExtraInstance2.Heartbeat(),
				})

				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.PendingStartMessages).Should(BeEmpty())

				Ω(outbox.PendingStopMessages).Should(HaveLen(3))

				expectedMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(3).InstanceGuid)
				Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), duplicateExtraInstance1.InstanceGuid)
				Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), duplicateExtraInstance2.InstanceGuid)
				Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))
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

			otherDesired := otherApp.DesiredState()
			otherDesired.NumberOfInstances = 3

			yetAnotherDesired := yetAnotherApp.DesiredState()
			yetAnotherDesired.NumberOfInstances = 2

			store.SaveDesiredState([]models.DesiredAppState{
				a.DesiredState(),
				otherDesired,
				yetAnotherDesired,
			})
			store.SaveActualState([]models.InstanceHeartbeat{
				a.InstanceAtIndex(0).Heartbeat(),
				a.InstanceAtIndex(1).Heartbeat(),
				undesiredApp.InstanceAtIndex(0).Heartbeat(),
				otherApp.InstanceAtIndex(0).Heartbeat(),
				otherApp.InstanceAtIndex(2).Heartbeat(),
			})
		})

		It("should analyze each app-version combination separately", func() {
			err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())

			Ω(outbox.PendingStartMessages).Should(HaveLen(3))

			expectedStartMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, otherApp.AppGuid, otherApp.AppVersion, 1, 1.0/3.0)
			Ω(outbox.PendingStartMessages).Should(ContainElement(EqualPendingStartMessage(expectedStartMessage)))

			expectedStartMessage = models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, yetAnotherApp.AppGuid, yetAnotherApp.AppVersion, 0, 1.0)
			Ω(outbox.PendingStartMessages).Should(ContainElement(EqualPendingStartMessage(expectedStartMessage)))

			expectedStartMessage = models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, yetAnotherApp.AppGuid, yetAnotherApp.AppVersion, 1, 1.0)
			Ω(outbox.PendingStartMessages).Should(ContainElement(EqualPendingStartMessage(expectedStartMessage)))

			Ω(outbox.PendingStopMessages).Should(HaveLen(2))

			expectedStopMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(1).InstanceGuid)
			Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedStopMessage)))

			expectedStopMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), undesiredApp.InstanceAtIndex(0).InstanceGuid)
			Ω(outbox.PendingStopMessages).Should(ContainElement(EqualPendingStopMessage(expectedStopMessage)))
		})
	})

	Context("When the store is not fresh", func() {
		BeforeEach(func() {
			store.Reset()

			desired := a.DesiredState()
			//this setup would, ordinarily, trigger a start and a stop
			store.SaveDesiredState([]models.DesiredAppState{
				desired,
			})
			store.SaveActualState([]models.InstanceHeartbeat{
				app.NewApp().InstanceAtIndex(0).Heartbeat(),
			})
		})

		Context("when the desired state is not fresh", func() {
			BeforeEach(func() {
				store.BumpActualFreshness(time.Unix(10, 0))
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err.Error()).Should(Equal("Desired state is not fresh"))
				Ω(outbox.PendingStartMessages).Should(BeEmpty())
				Ω(outbox.PendingStopMessages).Should(BeEmpty())
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
				Ω(outbox.PendingStartMessages).Should(BeEmpty())
				Ω(outbox.PendingStopMessages).Should(BeEmpty())
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
				Ω(outbox.PendingStartMessages).Should(BeEmpty())
				Ω(outbox.PendingStopMessages).Should(BeEmpty())
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
				Ω(outbox.PendingStartMessages).Should(BeEmpty())
				Ω(outbox.PendingStopMessages).Should(BeEmpty())
			})
		})
	})
})
