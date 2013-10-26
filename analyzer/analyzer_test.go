package analyzer_test

import (
	. "github.com/cloudfoundry/hm9000/analyzer"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"errors"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestoreadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"
	"time"
)

var _ = Describe("Analyzer", func() {
	var (
		analyzer     *Analyzer
		storeAdapter *fakestoreadapter.FakeStoreAdapter
		store        storepackage.Store
		timeProvider *faketimeprovider.FakeTimeProvider
		app          appfixture.AppFixture
	)

	conf, _ := config.DefaultConfig()

	BeforeEach(func() {
		storeAdapter = fakestoreadapter.New()
		store = storepackage.NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())

		timeProvider = &faketimeprovider.FakeTimeProvider{}
		timeProvider.TimeToProvide = time.Unix(1000, 0)

		app = appfixture.NewAppFixture()

		store.BumpActualFreshness(time.Unix(100, 0))
		store.BumpDesiredFreshness(time.Unix(100, 0))

		analyzer = New(store, timeProvider, fakelogger.NewFakeLogger(), conf)
	})

	startMessages := func() []models.PendingStartMessage {
		messages, _ := store.GetPendingStartMessages()
		messagesArr := []models.PendingStartMessage{}
		for _, message := range messages {
			messagesArr = append(messagesArr, message)
		}
		return messagesArr
	}

	stopMessages := func() []models.PendingStopMessage {
		messages, _ := store.GetPendingStopMessages()
		messagesArr := []models.PendingStopMessage{}
		for _, message := range messages {
			messagesArr = append(messagesArr, message)
		}
		return messagesArr
	}

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
				desired := app.DesiredState(3)
				desired.State = models.AppStateStarted
				store.SaveDesiredState(
					desired,
				)
				store.SaveActualState(
					app.InstanceAtIndex(0).Heartbeat(),
					app.InstanceAtIndex(1).Heartbeat(),
					app.InstanceAtIndex(2).Heartbeat(),
				)
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
				store.SaveDesiredState(
					app.DesiredState(2),
				)
			})

			Context("and none of the instances are running", func() {
				It("should send a start message for each of the missing instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(stopMessages()).Should(BeEmpty())
					Ω(startMessages()).Should(HaveLen(2))

					expectedMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 0, 1)
					Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))

					expectedMessage = models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 1, 1)
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
					existingMessage = models.NewPendingStartMessage(time.Unix(1, 0), 0, 0, app.AppGuid, app.AppVersion, 0, 0.5)
					store.SavePendingStartMessages(
						existingMessage,
					)
					analyzer.Analyze()
				})

				It("should not overwrite", func() {
					Ω(startMessages()).Should(HaveLen(2))
					Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(existingMessage)))
				})
			})

			Context("but only some of the instances are running", func() {
				BeforeEach(func() {
					store.SaveActualState(
						app.InstanceAtIndex(0).Heartbeat(),
					)
				})

				It("should return a start message containing only the missing indices", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(stopMessages()).Should(BeEmpty())

					Ω(startMessages()).Should(HaveLen(1))

					expectedMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 1, 0.5)
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
			store.SaveActualState(
				app.InstanceAtIndex(0).Heartbeat(),
				app.InstanceAtIndex(1).Heartbeat(),
				app.InstanceAtIndex(2).Heartbeat(),
			)
		})

		Context("when there are no desired instances", func() {
			It("should return an array of stop messages for the extra instances", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(HaveLen(3))

				expectedMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(0).InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(1).InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(2).InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))
			})
		})

		Context("when there is an existing stop message", func() {
			var existingMessage models.PendingStopMessage
			BeforeEach(func() {
				existingMessage = models.NewPendingStopMessage(time.Unix(1, 0), 0, 0, app.AppGuid, app.AppVersion, app.InstanceAtIndex(0).InstanceGuid)
				store.SavePendingStopMessages(
					existingMessage,
				)
				analyzer.Analyze()
			})

			It("should not overwrite", func() {
				Ω(stopMessages()).Should(HaveLen(3))
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(existingMessage)))
			})
		})

		Context("when the desired state requires fewer versions", func() {
			BeforeEach(func() {
				store.SaveDesiredState(
					app.DesiredState(1),
				)
			})

			Context("and all desired instances are present", func() {
				It("should return an array of stop messages for the (correct) extra instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(startMessages()).Should(BeEmpty())

					Ω(stopMessages()).Should(HaveLen(2))

					expectedMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(1).InstanceGuid)
					Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

					expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(2).InstanceGuid)
					Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))
				})
			})

			Context("and desired instances are missing", func() {
				BeforeEach(func() {
					storeAdapter.Delete("/apps/" + app.AppGuid + "-" + app.AppVersion + "/actual/" + app.InstanceAtIndex(0).InstanceGuid)
				})

				It("should return a start message containing the missing indices and no stop messages", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())

					Ω(startMessages()).Should(HaveLen(1))

					expectedMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 0, 1.0)
					Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))

					Ω(stopMessages()).Should(BeEmpty())
				})
			})
		})
	})

	Describe("Stopping duplicate instances (index < numDesired)", func() {
		var (
			duplicateInstance1 appfixture.Instance
			duplicateInstance2 appfixture.Instance
			duplicateInstance3 appfixture.Instance
		)

		BeforeEach(func() {
			store.SaveDesiredState(
				app.DesiredState(3),
			)

			duplicateInstance1 = app.InstanceAtIndex(2)
			duplicateInstance1.InstanceGuid = models.Guid()
			duplicateInstance2 = app.InstanceAtIndex(2)
			duplicateInstance2.InstanceGuid = models.Guid()
			duplicateInstance3 = app.InstanceAtIndex(2)
			duplicateInstance3.InstanceGuid = models.Guid()
		})

		Context("When there are missing instances on other indices", func() {
			It("should not schedule any stops and start the missing indices", func() {
				//[-,-,2|2|2|2]
				store.SaveActualState(
					app.InstanceAtIndex(2).Heartbeat(),
					duplicateInstance1.Heartbeat(),
					duplicateInstance2.Heartbeat(),
					duplicateInstance3.Heartbeat(),
				)

				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(stopMessages()).Should(BeEmpty())

				Ω(startMessages()).Should(HaveLen(2))

				expectedMessage := models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 0, 2.0/3.0)
				Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))

				expectedMessage = models.NewPendingStartMessage(timeProvider.Time(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 1, 2.0/3.0)
				Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedMessage)))
			})
		})

		Context("When all the other indices has instances", func() {
			BeforeEach(func() {
				//[0,1,2|2|2] < stop 2,2,2 with increasing delays etc...
				crashedHeartbeat := duplicateInstance3.Heartbeat()
				crashedHeartbeat.State = models.InstanceStateCrashed
				store.SaveActualState(
					app.InstanceAtIndex(0).Heartbeat(),
					app.InstanceAtIndex(1).Heartbeat(),
					app.InstanceAtIndex(2).Heartbeat(),
					duplicateInstance1.Heartbeat(),
					duplicateInstance2.Heartbeat(),
					crashedHeartbeat,
				)
			})

			It("should schedule a stop for every running instance at the duplicated index with increasing delays", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(startMessages()).Should(BeEmpty())

				Ω(stopMessages()).Should(HaveLen(3))

				instanceGuids := []string{}
				sendOns := []int{}
				for _, message := range stopMessages() {
					instanceGuids = append(instanceGuids, message.InstanceGuid)
					sendOns = append(sendOns, int(message.SendOn))
				}

				Ω(instanceGuids).Should(ContainElement(app.InstanceAtIndex(2).InstanceGuid))
				Ω(instanceGuids).Should(ContainElement(duplicateInstance1.InstanceGuid))
				Ω(instanceGuids).Should(ContainElement(duplicateInstance2.InstanceGuid))

				Ω(sendOns).Should(ContainElement(1000 + conf.GracePeriod()))
				Ω(sendOns).Should(ContainElement(1000 + conf.GracePeriod()*2))
				Ω(sendOns).Should(ContainElement(1000 + conf.GracePeriod()*3))
			})

			Context("when there is an existing stop message", func() {
				var existingMessage models.PendingStopMessage
				BeforeEach(func() {
					existingMessage = models.NewPendingStopMessage(time.Unix(1, 0), 0, 0, app.AppGuid, app.AppVersion, app.InstanceAtIndex(2).InstanceGuid)
					store.SavePendingStopMessages(
						existingMessage,
					)
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
				duplicateExtraInstance1 appfixture.Instance
				duplicateExtraInstance2 appfixture.Instance
			)

			BeforeEach(func() {
				duplicateExtraInstance1 = app.InstanceAtIndex(3)
				duplicateExtraInstance1.InstanceGuid = models.Guid()
				duplicateExtraInstance2 = app.InstanceAtIndex(3)
				duplicateExtraInstance2.InstanceGuid = models.Guid()
			})

			It("should terminate the extra indices with extreme prejudice", func() {
				//[0,1,2,3,3,3] < stop 3,3,3
				store.SaveActualState(
					app.InstanceAtIndex(0).Heartbeat(),
					app.InstanceAtIndex(1).Heartbeat(),
					app.InstanceAtIndex(2).Heartbeat(),
					app.InstanceAtIndex(3).Heartbeat(),
					duplicateExtraInstance1.Heartbeat(),
					duplicateExtraInstance2.Heartbeat(),
				)

				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(startMessages()).Should(BeEmpty())

				Ω(stopMessages()).Should(HaveLen(3))

				expectedMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(3).InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, duplicateExtraInstance1.InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, duplicateExtraInstance2.InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessage)))
			})
		})
	})

	Describe("Handling evacuating instances", func() {
		var evacuatingHeartbeat models.InstanceHeartbeat
		BeforeEach(func() {
			evacuatingHeartbeat = app.InstanceAtIndex(1).Heartbeat()
			evacuatingHeartbeat.State = models.InstanceStateEvacuating

			store.SaveActualState(
				app.InstanceAtIndex(0).Heartbeat(),
				evacuatingHeartbeat,
			)
		})

		Context("when the app is no longer desired", func() {
			It("should send an immediate stop", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())

				Ω(startMessages()).Should(BeEmpty())

				Ω(stopMessages()).Should(HaveLen(2))

				expectedMessageForIndex0 := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(0).InstanceGuid)
				expectedMessageForEvacuatingInstance := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, evacuatingHeartbeat.InstanceGuid)

				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessageForIndex0)))
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessageForEvacuatingInstance)))
			})
		})

		Context("when the EVACUTING instance is no longer in the desired range", func() {
			BeforeEach(func() {
				store.SaveDesiredState(app.DesiredState(1))
			})

			It("should send an immediate stop", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())

				Ω(startMessages()).Should(BeEmpty())

				Ω(stopMessages()).Should(HaveLen(1))

				expectedMessageForEvacuatingInstance := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, evacuatingHeartbeat.InstanceGuid)
				Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessageForEvacuatingInstance)))
			})
		})

		Context("and the index is still desired", func() {
			BeforeEach(func() {
				store.SaveDesiredState(app.DesiredState(2))
			})

			Context("and no other instances", func() {
				It("should schedule an immediate start message and no stop message", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())

					Ω(stopMessages()).Should(BeEmpty())

					Ω(startMessages()).Should(HaveLen(1))

					expectedMessageForIndex1 := models.NewPendingStartMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, 1, 2.0)
					Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedMessageForIndex1)))
				})
			})

			Context("when there is a RUNNING instance on the evacuating index", func() {
				BeforeEach(func() {
					heartbeat := app.InstanceAtIndex(1).Heartbeat()
					heartbeat.InstanceGuid = models.Guid()
					store.SaveActualState(heartbeat)
				})

				It("should schedule an immediate stop for the EVACUATING instance", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())

					Ω(startMessages()).Should(BeEmpty())

					Ω(stopMessages()).Should(HaveLen(1))

					expectedMessageForEvacuatingInstance := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, evacuatingHeartbeat.InstanceGuid)
					Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessageForEvacuatingInstance)))
				})

				Context("when there are multiple evacuating instances on the evacuating index", func() {
					var otherEvacuatingHeartbeat models.InstanceHeartbeat

					BeforeEach(func() {
						otherEvacuatingHeartbeat = app.InstanceAtIndex(1).Heartbeat()
						otherEvacuatingHeartbeat.InstanceGuid = models.Guid()
						otherEvacuatingHeartbeat.State = models.InstanceStateEvacuating
						store.SaveActualState(otherEvacuatingHeartbeat)
					})

					It("should schedule an immediate stop for both EVACUATING instances", func() {
						err := analyzer.Analyze()
						Ω(err).ShouldNot(HaveOccured())

						Ω(startMessages()).Should(BeEmpty())

						Ω(stopMessages()).Should(HaveLen(2))

						expectedMessageForEvacuatingInstance := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, evacuatingHeartbeat.InstanceGuid)
						Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessageForEvacuatingInstance)))
						expectedMessageForOtherEvacuatingInstance := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, otherEvacuatingHeartbeat.InstanceGuid)
						Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessageForOtherEvacuatingInstance)))
					})
				})
			})

			Context("when there is a STARTING instance on the evacuating index", func() {
				BeforeEach(func() {
					heartbeat := app.InstanceAtIndex(1).Heartbeat()
					heartbeat.InstanceGuid = models.Guid()
					heartbeat.State = models.InstanceStateStarting
					store.SaveActualState(heartbeat)
				})

				It("should not schedule anything", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())

					Ω(startMessages()).Should(BeEmpty())
					Ω(stopMessages()).Should(BeEmpty())
				})
			})

			Context("and the evacuating index's crash count exceeds NumberOfCrashesBeforeBackoffBegins", func() {
				BeforeEach(func() {
					store.SaveCrashCounts(models.CrashCount{
						AppGuid:       app.AppGuid,
						AppVersion:    app.AppVersion,
						InstanceIndex: 1,
						CrashCount:    conf.NumberOfCrashesBeforeBackoffBegins + 1,
					})
				})

				It("should schedule an immediate start message *and* a stop message for the EVACUATING instance", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())

					Ω(startMessages()).Should(HaveLen(1))
					expectedMessageForIndex1 := models.NewPendingStartMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, 1, 2.0)
					Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectedMessageForIndex1)))

					Ω(stopMessages()).Should(HaveLen(1))
					expectedMessageForEvacuatingInstance := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, evacuatingHeartbeat.InstanceGuid)
					Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedMessageForEvacuatingInstance)))
				})
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
				store.SaveActualState(
					app.CrashedInstanceHeartbeatAtIndex(0),
					app.CrashedInstanceHeartbeatAtIndex(0),
				)
			})

			Context("when the app is desired", func() {
				BeforeEach(func() {
					store.SaveDesiredState(
						app.DesiredState(1),
					)
				})

				It("should not try to stop crashed instances", func() {
					Ω(stopMessages()).Should(BeEmpty())
				})

				It("should try to start the instance immediately", func() {
					Ω(startMessages()).Should(HaveLen(1))
					expectMessage := models.NewPendingStartMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, 0, 1.0)
					Ω(startMessages()).Should(ContainElement(EqualPendingStartMessage(expectMessage)))
				})

				Context("when there is a running instance on the same index", func() {
					BeforeEach(func() {
						store.SaveActualState(app.InstanceAtIndex(0).Heartbeat())
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
				store.SaveActualState(
					app.CrashedInstanceHeartbeatAtIndex(0),
				)
				store.SaveDesiredState(
					app.DesiredState(1),
				)
			})

			It("should back off the scheduling", func() {
				expectedDelays := []int64{0, 0, 0, 30, 60, 120, 240, 480, 960, 960, 960}

				for _, expectedDelay := range expectedDelays {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(startMessages()[0].SendOn).Should(Equal(timeProvider.Time().Unix() + expectedDelay))
					store.DeletePendingStartMessages(startMessages()...)
				}
			})
		})

		Context("When all instances are crashed", func() {
			BeforeEach(func() {
				store.SaveActualState(
					app.CrashedInstanceHeartbeatAtIndex(0),
					app.CrashedInstanceHeartbeatAtIndex(1),
				)

				store.SaveDesiredState(
					app.DesiredState(2),
				)
			})

			It("should only try to start the index 0", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(startMessages()).Should(HaveLen(1))
				Ω(startMessages()[0].IndexToStart).Should(Equal(0))
			})
		})

		Context("When at least one instance is running and all others are crashed", func() {
			BeforeEach(func() {
				store.SaveActualState(
					app.CrashedInstanceHeartbeatAtIndex(0),
					app.CrashedInstanceHeartbeatAtIndex(1),
					app.InstanceAtIndex(2).Heartbeat(),
				)

				store.SaveDesiredState(
					app.DesiredState(3),
				)
			})

			It("should only try to start the index 0", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(startMessages()).Should(HaveLen(2))
				indexesToStart := []int{}
				for _, message := range startMessages() {
					indexesToStart = append(indexesToStart, message.IndexToStart)
				}
				Ω(indexesToStart).Should(ContainElement(0))
				Ω(indexesToStart).Should(ContainElement(1))
			})
		})
	})

	Describe("Processing multiple apps", func() {
		var (
			otherApp      appfixture.AppFixture
			yetAnotherApp appfixture.AppFixture
			undesiredApp  appfixture.AppFixture
		)

		BeforeEach(func() {
			otherApp = appfixture.NewAppFixture()
			undesiredApp = appfixture.NewAppFixture()
			yetAnotherApp = appfixture.NewAppFixture()
			undesiredApp.AppGuid = app.AppGuid

			store.SaveDesiredState(
				app.DesiredState(1),
				otherApp.DesiredState(3),
				yetAnotherApp.DesiredState(2),
			)
			store.SaveActualState(
				app.InstanceAtIndex(0).Heartbeat(),
				app.InstanceAtIndex(1).Heartbeat(),
				undesiredApp.InstanceAtIndex(0).Heartbeat(),
				otherApp.InstanceAtIndex(0).Heartbeat(),
				otherApp.InstanceAtIndex(2).Heartbeat(),
			)
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

			expectedStopMessage := models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(1).InstanceGuid)
			Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedStopMessage)))

			expectedStopMessage = models.NewPendingStopMessage(timeProvider.Time(), 0, conf.GracePeriod(), undesiredApp.AppGuid, undesiredApp.AppVersion, undesiredApp.InstanceAtIndex(0).InstanceGuid)
			Ω(stopMessages()).Should(ContainElement(EqualPendingStopMessage(expectedStopMessage)))
		})
	})

	Context("When the store is not fresh and/or fails to fetch data", func() {
		BeforeEach(func() {
			storeAdapter.Reset()

			desired := app.DesiredState(1)
			//this setup would, ordinarily, trigger a start and a stop
			store.SaveDesiredState(
				desired,
			)
			store.SaveActualState(
				appfixture.NewAppFixture().InstanceAtIndex(0).Heartbeat(),
			)
		})

		Context("when the desired state is not fresh", func() {
			BeforeEach(func() {
				store.BumpActualFreshness(time.Unix(10, 0))
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).Should(Equal(storepackage.DesiredIsNotFreshError))
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(BeEmpty())
			})
		})

		Context("when the actual state is not fresh", func() {
			BeforeEach(func() {
				store.BumpDesiredFreshness(time.Unix(10, 0))
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).Should(Equal(storepackage.ActualIsNotFreshError))
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(BeEmpty())
			})
		})

		Context("when the apps fail to fetch", func() {
			BeforeEach(func() {
				store.BumpActualFreshness(time.Unix(10, 0))
				store.BumpDesiredFreshness(time.Unix(10, 0))
				storeAdapter.ListErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("apps", errors.New("oops!"))
			})

			It("should return the store's error and not send any start/stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).Should(Equal(errors.New("oops!")))
				Ω(startMessages()).Should(BeEmpty())
				Ω(stopMessages()).Should(BeEmpty())
			})
		})
	})
})
