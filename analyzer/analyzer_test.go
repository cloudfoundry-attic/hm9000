package analyzer_test

import (
	. "github.com/cloudfoundry/hm9000/analyzer"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestore"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"
	"time"
)

var _ = Describe("Analyzer", func() {
	var (
		analyzer     *Analyzer
		store        *fakestore.FakeStore
		timeProvider *faketimeprovider.FakeTimeProvider
		a            app.App
	)

	conf, _ := config.DefaultConfig()

	BeforeEach(func() {
		store = fakestore.NewFakeStore()
		timeProvider = &faketimeprovider.FakeTimeProvider{}
		timeProvider.TimeToProvide = time.Unix(1000, 0)

		a = app.NewApp()

		store.BumpActualFreshness(time.Unix(100, 0))
		store.BumpDesiredFreshness(time.Unix(100, 0))

		analyzer = New(store, timeProvider, fakelogger.NewFakeLogger(), conf)
	})

	startMessages := func() []models.PendingStartMessage {
		messages, _ := store.GetPendingStartMessages()
		return messages
	}

	stopMessages := func() []models.PendingStopMessage {
		messages, _ := store.GetPendingStopMessages()
		return messages
	}

	Describe("Handling store errors", func() {
		Context("When fetching desired state fails with an error", func() {
			BeforeEach(func() {
				store.GetDesiredStateError = errors.New("oops!")
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).Should(Equal(errors.New("oops!")))
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(BeEmpty())
			})
		})

		Context("When fetching actual state fails with an error", func() {
			BeforeEach(func() {
				store.GetActualStateError = errors.New("oops!")
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).Should(Equal(errors.New("oops!")))
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(BeEmpty())
			})
		})
	})

	Describe("The steady state", func() {
		Context("When there are no desired or running apps", func() {
			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(BeEmpty())
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
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(BeEmpty())
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
					Ω(stopMessages()).Should(BeEmpty())
					Ω(startMessages()).Should(HaveLen(2))

					expectedMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 0, 1)
					Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))

					expectedMessage = models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 1, 1)
					Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))
				})

				It("should set the priority to 1", func() {
					analyzer.Analyze()
					for _, message := range startMessages() {
						Ω(message.Priority).Should(Equal(1.0))
					}
				})
			})

			Context("when there is an existing start message", func() {
				var existingMessage models.PendingStartMessage
				BeforeEach(func() {
					existingMessage = models.NewPendingStartMessage(time.Unix(1, 0), 0, 0, a.AppGuid, a.AppVersion, 0, 0.5)
					store.SavePendingStartMessages([]models.PendingStartMessage{
						existingMessage,
					})
					analyzer.Analyze()
				})

				It("should not overwrite", func() {
					Ω(startMessages()).Should(HaveLen(2))
					Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(existingMessage)))
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
					Ω(stopMessages()).Should(BeEmpty())

					Ω(startMessages()).Should(HaveLen(1))

					expectedMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 1, 0.5)
					Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))
				})

				It("should set the priority to 0.5", func() {
					analyzer.Analyze()
					for _, message := range startMessages() {
						Ω(message.Priority).Should(Equal(0.5))
					}
				})
			})
		})
	})

	Describe("Stopping extra instances (index >= numDesired)", func() {
		BeforeEach(func() {
			store.SaveActualState([]models.InstanceHeartbeat{
				a.InstanceAtIndex(0).Heartbeat(),
				a.InstanceAtIndex(1).Heartbeat(),
				a.InstanceAtIndex(2).Heartbeat(),
			})
		})

		Context("when there are no desired instances", func() {
			It("should return an array of stop messages for the extra instances", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(HaveLen(3))

				expectedMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(0).InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(1).InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(2).InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))
			})
		})

		Context("when there is an existing stop message", func() {
			var existingMessage models.PendingStopMessage
			BeforeEach(func() {
				existingMessage = models.NewPendingStopMessage(time.Unix(1, 0), 0, 0, a.InstanceAtIndex(0).InstanceGuid)
				store.SavePendingStopMessages([]models.PendingStopMessage{
					existingMessage,
				})
				analyzer.Analyze()
			})

			It("should not overwrite", func() {
				Ω(stopMessages()).Should(HaveLen(3))
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(existingMessage)))
			})
		})

		Context("when the desired state requires fewer versions", func() {
			BeforeEach(func() {
				desired := a.DesiredState()
				desired.NumberOfInstances = 1
				store.SaveDesiredState([]models.DesiredAppState{
					desired,
				})
			})

			Context("and all desired instances are present", func() {
				It("should return an array of stop messages for the (correct) extra instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(startMessages()).Should(BeEmpty())

					Ω(stopMessages()).Should(HaveLen(2))

					expectedMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(1).InstanceGuid)
					Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

					expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(2).InstanceGuid)
					Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))
				})
			})

			Context("and desired instances are missing", func() {
				BeforeEach(func() {
					store.DeleteActualState([]models.InstanceHeartbeat{a.InstanceAtIndex(0).Heartbeat()})
				})

				It("should return a start message containing the missing indices and no stop messages", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())

					Ω(startMessages()).Should(HaveLen(1))

					expectedMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 0, 1.0)
					Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))

					Ω(stopMessages()).Should(BeEmpty())
				})
			})
		})
	})

	Describe("Stopping duplicate instances (index < numDesired)", func() {
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
				Ω(stopMessages()).Should(BeEmpty())

				Ω(startMessages()).Should(HaveLen(2))

				expectedMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 0, 2.0/3.0)
				Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))

				expectedMessage = models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, a.AppGuid, a.AppVersion, 1, 2.0/3.0)
				Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))
			})
		})

		Context("When all the other indices has instances", func() {
			BeforeEach(func() {
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
			})

			It("should schedule a stop for every running instance at the duplicated index with increasing delays", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(startMessages()).Should(BeEmpty())

				Ω(stopMessages()).Should(HaveLen(3))

				expectedMessage := models.NewPendingStopMessage(timeProvider.Time(), conf.GracePeriod(), conf.GracePeriod(), a.InstanceAtIndex(2).InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), conf.GracePeriod()*2, conf.GracePeriod(), duplicateInstance1.InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), conf.GracePeriod()*3, conf.GracePeriod(), duplicateInstance2.InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))
			})

			Context("when there is an existing stop message", func() {
				var existingMessage models.PendingStopMessage
				BeforeEach(func() {
					existingMessage = models.NewPendingStopMessage(time.Unix(1, 0), 0, 0, a.InstanceAtIndex(2).InstanceGuid)
					store.SavePendingStopMessages([]models.PendingStopMessage{
						existingMessage,
					})
					analyzer.Analyze()
				})

				It("should not overwrite", func() {
					Ω(stopMessages()).Should(HaveLen(3))
					Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(existingMessage)))
				})
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
				Ω(startMessages()).Should(BeEmpty())

				Ω(stopMessages()).Should(HaveLen(3))

				expectedMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(3).InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), duplicateExtraInstance1.InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), duplicateExtraInstance2.InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))
			})
		})
	})

	Describe("Handling crashed instances", func() {
		Context("When there are multiple crashed instances on the same index", func() {
			JustBeforeEach(func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
			})

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
					Ω(stopMessages()).Should(BeEmpty())
				})

				It("should try to start the instance immediately", func() {
					Ω(startMessages()).Should(HaveLen(1))
					expectMessage := models.NewPendingStartMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.AppGuid, a.AppVersion, 0, 1.0)
					Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectMessage)))
				})

				Context("when there is a running instance on the same index", func() {
					BeforeEach(func() {
						store.SaveActualState([]models.InstanceHeartbeat{a.InstanceAtIndex(0).Heartbeat()})
					})

					It("should not try to stop the running instance!", func() {
						Ω(stopMessages()).Should(BeEmpty())
					})

					It("should not try to start the instance", func() {
						Ω(startMessages()).Should(BeEmpty())
					})
				})
			})

			Context("when the app is not desired", func() {
				It("should not try to stop crashed instances", func() {
					Ω(stopMessages()).Should(BeEmpty())
				})

				It("should not try to start the instance", func() {
					Ω(startMessages()).Should(BeEmpty())
				})
			})
		})

		Describe("applying the backoff", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.CrashedInstanceHeartbeatAtIndex(0),
				})
				store.SaveDesiredState([]models.DesiredAppState{
					a.DesiredState(),
				})
			})

			It("should back off the scheduling", func() {
				expectedDelays := []int64{0, 0, 0, 30, 60, 120, 240, 480, 960, 960, 960}

				for _, expectedDelay := range expectedDelays {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(startMessages()[0].SendOn).Should(Equal(timeProvider.Time().Unix() + expectedDelay))
					store.DeletePendingStartMessages(startMessages())
				}
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

			Ω(startMessages()).Should(HaveLen(3))

			expectedStartMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, otherApp.AppGuid, otherApp.AppVersion, 1, 1.0/3.0)
			Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedStartMessage)))

			expectedStartMessage = models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, yetAnotherApp.AppGuid, yetAnotherApp.AppVersion, 0, 1.0)
			Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedStartMessage)))

			expectedStartMessage = models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, yetAnotherApp.AppGuid, yetAnotherApp.AppVersion, 1, 1.0)
			Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedStartMessage)))

			Ω(stopMessages()).Should(HaveLen(2))

			expectedStopMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), a.InstanceAtIndex(1).InstanceGuid)
			Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedStopMessage)))

			expectedStopMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), undesiredApp.InstanceAtIndex(0).InstanceGuid)
			Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedStopMessage)))
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
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(BeEmpty())
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
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(BeEmpty())
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
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(BeEmpty())
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
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(BeEmpty())
			})
		})
	})
})
