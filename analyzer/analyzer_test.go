package analyzer_test

import (
	. "github.com/cloudfoundry/hm9000/analyzer"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/clock/fakeclock"

	"errors"
	"time"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
)

var _ = Describe("Analyzer", func() {
	var (
		analyzer     *Analyzer
		storeAdapter *fakestoreadapter.FakeStoreAdapter
		store        storepackage.Store
		clock        *fakeclock.FakeClock
		dea          appfixture.DeaFixture
		app          appfixture.AppFixture
		appQueue     *models.AppQueue
		done         chan struct{}
	)

	conf, _ := config.DefaultConfig()

	BeforeEach(func() {
		storeAdapter = fakestoreadapter.New()
		store = storepackage.NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())

		clock = fakeclock.NewFakeClock(time.Unix(1000, 0))

		dea = appfixture.NewDeaFixture()
		app = dea.GetApp(0)

		store.BumpActualFreshness(time.Unix(100, 0))

		appQueue = models.NewAppQueue()
		appQueue.SetFetchDesiredAppsSuccess(true)

		analyzer = New(store, clock, fakelogger.NewFakeLogger(), conf)

		done = make(chan struct{})
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

	writeToQueue := func(desiredStateData map[string]models.DesiredAppState) {
		go func() {
			appQueue.DesiredApps <- desiredStateData
			close(appQueue.DesiredApps)
			close(done)
		}()
	}

	It("sets the appQueue analyzing state to false on completion", func() {
		close(appQueue.DesiredApps)
		analyzer.Analyze(appQueue)

		Expect(appQueue.DoneAnalyzing).To(BeClosed())
	})

	Describe("The steady state", func() {
		Context("When there are no desired or running apps", func() {
			BeforeEach(func() {
				close(appQueue.DesiredApps)
			})

			It("should not send any start or stop messages", func() {
				_, _, _, err := analyzer.Analyze(appQueue)

				Expect(err).ToNot(HaveOccurred())
				Expect(startMessages()).To(BeEmpty())
				Expect(stopMessages()).To(BeEmpty())
			})
		})

		Context("When the desired number of instances and the running number of instances match", func() {
			BeforeEach(func() {
				desired := app.DesiredState(3)
				desired.State = models.AppStateStarted
				desiredStateData := make(map[string]models.DesiredAppState)
				desiredStateData[desired.StoreKey()] = desired

				writeToQueue(desiredStateData)

				store.SyncHeartbeats(app.Heartbeat(3))
			})

			AfterEach(func() {
				Eventually(done).Should(BeClosed())
			})

			It("should not send any start or stop messages", func() {
				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).ToNot(HaveOccurred())
				Expect(startMessages()).To(BeEmpty())
				Expect(stopMessages()).To(BeEmpty())
			})
		})
	})

	Describe("Starting missing instances", func() {
		Context("where an staged app has desired instances", func() {
			BeforeEach(func() {
				desired := app.DesiredState(2)
				desiredStateData := make(map[string]models.DesiredAppState)
				desiredStateData[desired.StoreKey()] = desired
				writeToQueue(desiredStateData)
			})

			AfterEach(func() {
				Eventually(done).Should(BeClosed())
			})

			Context("and none of the instances are running", func() {
				It("should send a start message for each of the missing instances", func() {
					_, _, _, err := analyzer.Analyze(appQueue)
					Expect(err).ToNot(HaveOccurred())
					Expect(stopMessages()).To(BeEmpty())
					Expect(startMessages()).To(HaveLen(2))

					expectedMessage := models.NewPendingStartMessage(clock.Now(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 0, 1, models.PendingStartMessageReasonMissing)
					Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedMessage)))

					expectedMessage = models.NewPendingStartMessage(clock.Now(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 1, 1, models.PendingStartMessageReasonMissing)
					Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedMessage)))
				})

				It("should set the priority to 1", func() {
					analyzer.Analyze(appQueue)
					for _, message := range startMessages() {
						Expect(message.Priority).To(Equal(1.0))
					}
				})
			})

			Context("and one of the instances is evacuating", func() {
				BeforeEach(func() {
					evacuatingHeartbeat := app.InstanceAtIndex(0).Heartbeat()
					evacuatingHeartbeat.State = models.InstanceStateEvacuating

					store.SyncHeartbeats(dea.HeartbeatWith(
						evacuatingHeartbeat,
					))
				})

				It("should send a start message with the correct reason", func() {
					_, _, _, _, err := analyzer.Analyze(appQueue)
					Expect(err).ToNot(HaveOccurred())
					Expect(stopMessages()).To(BeEmpty())
					Expect(startMessages()).To(HaveLen(2))

					expectedMessage := models.NewPendingStartMessage(clock.Now(), 0, 30, app.AppGuid, app.AppVersion, 0, 2, models.PendingStartMessageReasonEvacuating)
					Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedMessage)))

					expectedMessage = models.NewPendingStartMessage(clock.Now(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 1, 1, models.PendingStartMessageReasonMissing)
					Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedMessage)))
				})
			})

			Context("when there is an existing start message", func() {
				var existingMessage models.PendingStartMessage
				BeforeEach(func() {
					existingMessage = models.NewPendingStartMessage(time.Unix(1, 0), 0, 0, app.AppGuid, app.AppVersion, 0, 1, models.PendingStartMessageReasonMissing)
					store.SavePendingStartMessages(
						existingMessage,
					)
					Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(existingMessage)))
					analyzer.Analyze(appQueue)
				})

				It("should not overwrite", func() {
					Expect(startMessages()).To(HaveLen(2))
					Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(existingMessage)))
				})
			})

			Context("but only some of the instances are running", func() {
				BeforeEach(func() {
					store.SyncHeartbeats(app.Heartbeat(1))
				})

				It("should return a start message containing only the missing indices", func() {
					_, _, _, err := analyzer.Analyze(appQueue)
					Expect(err).ToNot(HaveOccurred())
					Expect(stopMessages()).To(BeEmpty())

					Expect(startMessages()).To(HaveLen(1))

					expectedMessage := models.NewPendingStartMessage(clock.Now(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 1, 0.5, models.PendingStartMessageReasonMissing)
					Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedMessage)))
				})

				It("should set the priority to 0.5", func() {
					analyzer.Analyze(appQueue)
					for _, message := range startMessages() {
						Expect(message.Priority).To(Equal(0.5))
					}
				})
			})
		})

		Context("When the app has not finished staging and has desired instances", func() {
			BeforeEach(func() {
				desiredState := app.DesiredState(2)
				desiredState.PackageState = models.AppPackageStatePending
				desiredStateData := make(map[string]models.DesiredAppState)
				desiredStateData[desiredState.StoreKey()] = desiredState
				writeToQueue(desiredStateData)
			})

			AfterEach(func() {
				Eventually(done).Should(BeClosed())
			})

			It("should not start any missing instances", func() {
				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).ToNot(HaveOccurred())
				Expect(stopMessages()).To(BeEmpty())
				Expect(startMessages()).To(BeEmpty())
			})
		})
	})

	Describe("Stopping extra instances (index >= numDesired)", func() {
		BeforeEach(func() {
			store.SyncHeartbeats(app.Heartbeat(3))
		})

		Context("when there are no desired instances", func() {
			It("should return an array of stop messages for the extra instances", func() {
				close(appQueue.DesiredApps)
				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).ToNot(HaveOccurred())
				Expect(startMessages()).To(BeEmpty())
				Expect(stopMessages()).To(HaveLen(3))

				expectedMessage := models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(0).InstanceGuid, models.PendingStopMessageReasonExtra)
				Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(1).InstanceGuid, models.PendingStopMessageReasonExtra)
				Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(2).InstanceGuid, models.PendingStopMessageReasonExtra)
				Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessage)))
			})
		})

		Context("when there is an existing stop message", func() {
			var existingMessage models.PendingStopMessage
			BeforeEach(func() {
				existingMessage = models.NewPendingStopMessage(time.Unix(1, 0), 0, 0, app.AppGuid, app.AppVersion, app.InstanceAtIndex(0).InstanceGuid, models.PendingStopMessageReasonExtra)
				store.SavePendingStopMessages(
					existingMessage,
				)
				close(appQueue.DesiredApps)
				analyzer.Analyze(appQueue)
			})

			It("should not overwrite", func() {
				Expect(stopMessages()).To(HaveLen(3))
				Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(existingMessage)))
			})
		})

		Context("when the desired state requires fewer versions", func() {
			BeforeEach(func() {
				desiredState := app.DesiredState(1)
				desiredStateData := make(map[string]models.DesiredAppState)
				desiredStateData[desiredState.StoreKey()] = desiredState
				writeToQueue(desiredStateData)
			})

			AfterEach(func() {
				Eventually(done).Should(BeClosed())
			})

			Context("and all desired instances are present", func() {
				It("should return an array of stop messages for the (correct) extra instances", func() {
					_, _, _, err := analyzer.Analyze(appQueue)
					Expect(err).ToNot(HaveOccurred())
					Expect(startMessages()).To(BeEmpty())

					Expect(stopMessages()).To(HaveLen(2))

					expectedMessage := models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(1).InstanceGuid, models.PendingStopMessageReasonExtra)
					Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessage)))

					expectedMessage = models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(2).InstanceGuid, models.PendingStopMessageReasonExtra)
					Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessage)))
				})
			})

			Context("and desired instances are missing", func() {
				BeforeEach(func() {
					storeAdapter.Delete("/hm/v1/apps/actual/" + app.AppGuid + "," + app.AppVersion + "/" + app.InstanceAtIndex(0).InstanceGuid)
				})

				It("should return a start message containing the missing indices and no stop messages", func() {
					_, _, _, err := analyzer.Analyze(appQueue)
					Expect(err).ToNot(HaveOccurred())

					Expect(startMessages()).To(HaveLen(1))

					expectedMessage := models.NewPendingStartMessage(clock.Now(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 0, 1.0, models.PendingStartMessageReasonMissing)
					Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedMessage)))

					Expect(stopMessages()).To(BeEmpty())
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
			desiredState := app.DesiredState(3)
			desiredStateData := make(map[string]models.DesiredAppState)
			desiredStateData[desiredState.StoreKey()] = desiredState
			writeToQueue(desiredStateData)

			duplicateInstance1 = app.InstanceAtIndex(2)
			duplicateInstance1.InstanceGuid = models.Guid()
			duplicateInstance2 = app.InstanceAtIndex(2)
			duplicateInstance2.InstanceGuid = models.Guid()
			duplicateInstance3 = app.InstanceAtIndex(2)
			duplicateInstance3.InstanceGuid = models.Guid()
		})

		AfterEach(func() {
			Eventually(done).Should(BeClosed())
		})

		Context("When there are missing instances on other indices", func() {
			It("should not schedule any stops but should start the missing indices", func() {
				//[-,-,2|2|2|2]
				store.SyncHeartbeats(dea.HeartbeatWith(
					app.InstanceAtIndex(2).Heartbeat(),
					duplicateInstance1.Heartbeat(),
					duplicateInstance2.Heartbeat(),
					duplicateInstance3.Heartbeat(),
				))

				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).ToNot(HaveOccurred())
				Expect(stopMessages()).To(BeEmpty())

				Expect(startMessages()).To(HaveLen(2))

				expectedMessage := models.NewPendingStartMessage(clock.Now(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 0, 2.0/3.0, models.PendingStartMessageReasonMissing)
				Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedMessage)))

				expectedMessage = models.NewPendingStartMessage(clock.Now(), conf.GracePeriod(), 0, app.AppGuid, app.AppVersion, 1, 2.0/3.0, models.PendingStartMessageReasonMissing)
				Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedMessage)))
			})
		})

		Context("When all the other indices has instances", func() {
			BeforeEach(func() {
				//[0,1,2|2|2] < stop 2,2,2 with increasing delays etc...
				crashedHeartbeat := duplicateInstance3.Heartbeat()
				crashedHeartbeat.State = models.InstanceStateCrashed
				store.SyncHeartbeats(dea.HeartbeatWith(
					app.InstanceAtIndex(0).Heartbeat(),
					app.InstanceAtIndex(1).Heartbeat(),
					app.InstanceAtIndex(2).Heartbeat(),
					duplicateInstance1.Heartbeat(),
					duplicateInstance2.Heartbeat(),
					crashedHeartbeat,
				))
			})

			It("should schedule a stop for every running instance at the duplicated index with increasing delays", func() {
				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).ToNot(HaveOccurred())
				Expect(startMessages()).To(BeEmpty())

				Expect(stopMessages()).To(HaveLen(3))

				instanceGuids := []string{}
				sendOns := []int{}
				for _, message := range stopMessages() {
					instanceGuids = append(instanceGuids, message.InstanceGuid)
					sendOns = append(sendOns, int(message.SendOn))
					Expect(message.StopReason).To(Equal(models.PendingStopMessageReasonDuplicate))
				}

				Expect(instanceGuids).To(ContainElement(app.InstanceAtIndex(2).InstanceGuid))
				Expect(instanceGuids).To(ContainElement(duplicateInstance1.InstanceGuid))
				Expect(instanceGuids).To(ContainElement(duplicateInstance2.InstanceGuid))

				Expect(sendOns).To(ContainElement(1000 + conf.GracePeriod()*4))
				Expect(sendOns).To(ContainElement(1000 + conf.GracePeriod()*5))
				Expect(sendOns).To(ContainElement(1000 + conf.GracePeriod()*6))
			})

			Context("when there is an existing stop message", func() {
				var existingMessage models.PendingStopMessage
				BeforeEach(func() {
					existingMessage = models.NewPendingStopMessage(time.Unix(1, 0), 0, 0, app.AppGuid, app.AppVersion, app.InstanceAtIndex(2).InstanceGuid, models.PendingStopMessageReasonDuplicate)
					store.SavePendingStopMessages(
						existingMessage,
					)
					analyzer.Analyze(appQueue)
				})

				It("should not overwrite", func() {
					Expect(stopMessages()).To(HaveLen(3))
					Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(existingMessage)))
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
				store.SyncHeartbeats(dea.HeartbeatWith(
					app.InstanceAtIndex(0).Heartbeat(),
					app.InstanceAtIndex(1).Heartbeat(),
					app.InstanceAtIndex(2).Heartbeat(),
					app.InstanceAtIndex(3).Heartbeat(),
					duplicateExtraInstance1.Heartbeat(),
					duplicateExtraInstance2.Heartbeat(),
				))

				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).ToNot(HaveOccurred())
				Expect(startMessages()).To(BeEmpty())

				Expect(stopMessages()).To(HaveLen(3))

				expectedMessage := models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(3).InstanceGuid, models.PendingStopMessageReasonExtra)
				Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, duplicateExtraInstance1.InstanceGuid, models.PendingStopMessageReasonExtra)
				Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessage)))

				expectedMessage = models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, duplicateExtraInstance2.InstanceGuid, models.PendingStopMessageReasonExtra)
				Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessage)))
			})
		})
	})

	Describe("Handling evacuating instances", func() {
		var heartbeat *models.Heartbeat
		var evacuatingHeartbeat models.InstanceHeartbeat
		BeforeEach(func() {
			evacuatingHeartbeat = app.InstanceAtIndex(1).Heartbeat()
			evacuatingHeartbeat.State = models.InstanceStateEvacuating
			heartbeat = dea.HeartbeatWith(
				app.InstanceAtIndex(0).Heartbeat(),
				evacuatingHeartbeat,
			)
			store.SyncHeartbeats(heartbeat)
		})

		Context("when the app is no longer desired", func() {

			It("should send an immediate stop", func() {
				close(appQueue.DesiredApps)
				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).ToNot(HaveOccurred())

				Expect(startMessages()).To(BeEmpty())

				Expect(stopMessages()).To(HaveLen(2))

				expectedMessageForIndex0 := models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(0).InstanceGuid, models.PendingStopMessageReasonExtra)
				expectedMessageForEvacuatingInstance := models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, evacuatingHeartbeat.InstanceGuid, models.PendingStopMessageReasonExtra)

				Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessageForIndex0)))
				Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessageForEvacuatingInstance)))
			})
		})

		Context("when the app hasn't finished staging", func() {
			BeforeEach(func() {
				desired := app.DesiredState(2)
				desired.PackageState = models.AppPackageStatePending
				desiredStateData := make(map[string]models.DesiredAppState)
				desiredStateData[desired.StoreKey()] = desired
				writeToQueue(desiredStateData)
			})

			AfterEach(func() {
				Eventually(done).Should(BeClosed())
			})

			It("should send an immediate start & stop (as the EVACUATING instance cannot be started on another DEA, but we're going to try...)", func() {
				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).ToNot(HaveOccurred())

				Expect(startMessages()).To(HaveLen(1))
				Expect(stopMessages()).To(HaveLen(1))

				expectedMessageForIndex1 := models.NewPendingStartMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, 1, 2.0, models.PendingStartMessageReasonEvacuating)
				Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedMessageForIndex1)))

				expectedMessageForEvacuatingInstance := models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, evacuatingHeartbeat.InstanceGuid, models.PendingStopMessageReasonEvacuationComplete)
				Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessageForEvacuatingInstance)))
			})
		})

		Context("when the EVACUTING instance is no longer in the desired range", func() {
			BeforeEach(func() {
				desiredState := app.DesiredState(1)
				desiredStateData := make(map[string]models.DesiredAppState)
				desiredStateData[desiredState.StoreKey()] = desiredState
				writeToQueue(desiredStateData)
			})

			AfterEach(func() {
				Eventually(done).Should(BeClosed())
			})

			It("should send an immediate stop", func() {
				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).ToNot(HaveOccurred())

				Expect(startMessages()).To(BeEmpty())

				Expect(stopMessages()).To(HaveLen(1))

				expectedMessageForEvacuatingInstance := models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, evacuatingHeartbeat.InstanceGuid, models.PendingStopMessageReasonExtra)
				Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessageForEvacuatingInstance)))
			})
		})

		Context("and the index is still desired", func() {
			BeforeEach(func() {
				desiredState := app.DesiredState(2)
				desiredStateData := make(map[string]models.DesiredAppState)
				desiredStateData[desiredState.StoreKey()] = desiredState
				writeToQueue(desiredStateData)
			})

			AfterEach(func() {
				Eventually(done).Should(BeClosed())
			})

			Context("and no other instances", func() {
				It("should schedule an immediate start message and no stop message", func() {
					_, _, _, err := analyzer.Analyze(appQueue)
					Expect(err).ToNot(HaveOccurred())

					Expect(stopMessages()).To(BeEmpty())

					Expect(startMessages()).To(HaveLen(1))

					expectedMessageForIndex1 := models.NewPendingStartMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, 1, 2.0, models.PendingStartMessageReasonEvacuating)
					Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedMessageForIndex1)))
				})
			})

			Context("when there is a RUNNING instance on the evacuating index", func() {
				BeforeEach(func() {
					runningInstanceHeartbeat := app.InstanceAtIndex(1).Heartbeat()
					runningInstanceHeartbeat.InstanceGuid = models.Guid()
					heartbeat.InstanceHeartbeats = append(heartbeat.InstanceHeartbeats, runningInstanceHeartbeat)
					store.SyncHeartbeats(heartbeat)
				})

				It("should schedule an immediate stop for the EVACUATING instance", func() {
					_, _, _, err := analyzer.Analyze(appQueue)
					Expect(err).ToNot(HaveOccurred())

					Expect(startMessages()).To(BeEmpty())

					Expect(stopMessages()).To(HaveLen(1))

					expectedMessageForEvacuatingInstance := models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, evacuatingHeartbeat.InstanceGuid, models.PendingStopMessageReasonEvacuationComplete)
					Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessageForEvacuatingInstance)))
				})

				Context("when there are multiple evacuating instances on the evacuating index", func() {
					var otherEvacuatingHeartbeat models.InstanceHeartbeat

					BeforeEach(func() {
						otherEvacuatingHeartbeat = app.InstanceAtIndex(1).Heartbeat()
						otherEvacuatingHeartbeat.InstanceGuid = models.Guid()
						otherEvacuatingHeartbeat.State = models.InstanceStateEvacuating
						heartbeat.InstanceHeartbeats = append(heartbeat.InstanceHeartbeats, otherEvacuatingHeartbeat)
						store.SyncHeartbeats(heartbeat)
					})

					It("should schedule an immediate stop for both EVACUATING instances", func() {
						_, _, _, err := analyzer.Analyze(appQueue)
						Expect(err).ToNot(HaveOccurred())

						Expect(startMessages()).To(BeEmpty())

						Expect(stopMessages()).To(HaveLen(2))

						expectedMessageForEvacuatingInstance := models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, evacuatingHeartbeat.InstanceGuid, models.PendingStopMessageReasonEvacuationComplete)
						Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessageForEvacuatingInstance)))
						expectedMessageForOtherEvacuatingInstance := models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, otherEvacuatingHeartbeat.InstanceGuid, models.PendingStopMessageReasonEvacuationComplete)
						Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessageForOtherEvacuatingInstance)))
					})
				})
			})

			Context("when there is a STARTING instance on the evacuating index", func() {
				BeforeEach(func() {
					startingInstanceHeartbeat := app.InstanceAtIndex(1).Heartbeat()
					startingInstanceHeartbeat.InstanceGuid = models.Guid()
					startingInstanceHeartbeat.State = models.InstanceStateStarting
					heartbeat.InstanceHeartbeats = append(heartbeat.InstanceHeartbeats, startingInstanceHeartbeat)
					store.SyncHeartbeats(heartbeat)
				})

				It("should not schedule anything", func() {
					_, _, _, err := analyzer.Analyze(appQueue)
					Expect(err).ToNot(HaveOccurred())

					Expect(startMessages()).To(BeEmpty())
					Expect(stopMessages()).To(BeEmpty())
				})
			})

			Context("and the evacuating index's crash count is greater than 0", func() {
				BeforeEach(func() {
					store.SaveCrashCounts(models.CrashCount{
						AppGuid:       app.AppGuid,
						AppVersion:    app.AppVersion,
						InstanceIndex: 1,
						CrashCount:    1,
					})
				})

				It("should schedule an immediate start message *and* a stop message for the EVACUATING instance", func() {
					_, _, _, err := analyzer.Analyze(appQueue)
					Expect(err).ToNot(HaveOccurred())

					Expect(startMessages()).To(HaveLen(1))
					expectedMessageForIndex1 := models.NewPendingStartMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, 1, 2.0, models.PendingStartMessageReasonEvacuating)
					Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedMessageForIndex1)))

					Expect(stopMessages()).To(HaveLen(1))
					expectedMessageForEvacuatingInstance := models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, evacuatingHeartbeat.InstanceGuid, models.PendingStopMessageReasonEvacuationComplete)
					Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedMessageForEvacuatingInstance)))
				})
			})
		})
	})

	Describe("Handling crashed instances", func() {
		var heartbeat *models.Heartbeat
		Context("When there are multiple crashed instances on the same index", func() {
			JustBeforeEach(func() {
				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).ToNot(HaveOccurred())
			})

			BeforeEach(func() {
				heartbeat = dea.HeartbeatWith(app.CrashedInstanceHeartbeatAtIndex(0), app.CrashedInstanceHeartbeatAtIndex(0))
				store.SyncHeartbeats(heartbeat)
			})

			Context("when the app is desired", func() {
				BeforeEach(func() {
					desiredState := app.DesiredState(1)
					desiredStateData := make(map[string]models.DesiredAppState)
					desiredStateData[desiredState.StoreKey()] = desiredState
					writeToQueue(desiredStateData)
				})

				AfterEach(func() {
					Eventually(done).Should(BeClosed())
				})

				It("should not try to stop crashed instances", func() {
					Expect(stopMessages()).To(BeEmpty())
				})

				It("should try to start the instance immediately", func() {
					Expect(startMessages()).To(HaveLen(1))
					expectMessage := models.NewPendingStartMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, 0, 1.0, models.PendingStartMessageReasonCrashed)
					Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectMessage)))
				})

				Context("when there is a running instance on the same index", func() {
					BeforeEach(func() {
						heartbeat.InstanceHeartbeats = append(heartbeat.InstanceHeartbeats, app.InstanceAtIndex(0).Heartbeat())
						store.SyncHeartbeats(heartbeat)
					})

					It("should not try to stop the running instance!", func() {
						Expect(stopMessages()).To(BeEmpty())
					})

					It("should not try to start the instance", func() {
						Expect(startMessages()).To(BeEmpty())
					})
				})
			})

			Context("when the app is not desired", func() {
				BeforeEach(func() {
					close(appQueue.DesiredApps)
				})
				It("should not try to stop crashed instances", func() {
					Expect(stopMessages()).To(BeEmpty())
				})

				It("should not try to start the instance", func() {
					Expect(startMessages()).To(BeEmpty())
				})
			})

			Context("when the app package state is pending", func() {
				BeforeEach(func() {
					desiredState := app.DesiredState(1)
					desiredState.PackageState = models.AppPackageStatePending
					desiredStateData := make(map[string]models.DesiredAppState)
					desiredStateData[desiredState.StoreKey()] = desiredState
					writeToQueue(desiredStateData)
				})

				AfterEach(func() {
					Eventually(done).Should(BeClosed())
				})

				It("should not try to start/stop the instance", func() {
					Expect(stopMessages()).To(BeEmpty())
					Expect(startMessages()).To(BeEmpty())
				})
			})
		})

		Describe("applying the backoff", func() {
			BeforeEach(func() {
				heartbeat = dea.HeartbeatWith(app.CrashedInstanceHeartbeatAtIndex(0))
				store.SyncHeartbeats(heartbeat)
			})

			It("should back off the scheduling", func() {
				expectedDelays := []int64{0, 0, 0, 30, 60, 120, 240, 480, 960, 960, 960}

				for _, expectedDelay := range expectedDelays {
					done = make(chan struct{})

					appQueue = models.NewAppQueue()
					appQueue.SetFetchDesiredAppsSuccess(true)
					desiredState := app.DesiredState(1)
					desiredStateData := make(map[string]models.DesiredAppState)
					desiredStateData[desiredState.StoreKey()] = desiredState
					writeToQueue(desiredStateData)

					_, _, _, err := analyzer.Analyze(appQueue)
					Expect(err).ToNot(HaveOccurred())
					Expect(startMessages()[0].SendOn).To(Equal(clock.Now().Unix() + expectedDelay))
					Expect(startMessages()[0].StartReason).To(Equal(models.PendingStartMessageReasonCrashed))
					store.DeletePendingStartMessages(startMessages()...)
					Eventually(done).Should(BeClosed())
				}
			})
		})

		Context("When all instances are crashed", func() {
			BeforeEach(func() {
				heartbeat = dea.HeartbeatWith(app.CrashedInstanceHeartbeatAtIndex(0), app.CrashedInstanceHeartbeatAtIndex(1))
				store.SyncHeartbeats(heartbeat)

				desiredState := app.DesiredState(2)
				desiredStateData := make(map[string]models.DesiredAppState)
				desiredStateData[desiredState.StoreKey()] = desiredState
				writeToQueue(desiredStateData)
			})

			AfterEach(func() {
				Eventually(done).Should(BeClosed())
			})

			It("should only try to start the index 0", func() {
				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).ToNot(HaveOccurred())
				Expect(startMessages()).To(HaveLen(1))
				Expect(startMessages()[0].IndexToStart).To(Equal(0))
			})
		})

		Context("When at least one instance is running and all others are crashed", func() {
			BeforeEach(func() {
				heartbeat = dea.HeartbeatWith(app.CrashedInstanceHeartbeatAtIndex(0), app.CrashedInstanceHeartbeatAtIndex(1), app.InstanceAtIndex(2).Heartbeat())
				store.SyncHeartbeats(heartbeat)

				desiredState := app.DesiredState(3)
				desiredStateData := make(map[string]models.DesiredAppState)
				desiredStateData[desiredState.StoreKey()] = desiredState
				writeToQueue(desiredStateData)
			})

			AfterEach(func() {
				Eventually(done).Should(BeClosed())
			})

			It("should only try to start the index 0", func() {
				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).ToNot(HaveOccurred())
				Expect(startMessages()).To(HaveLen(2))
				indexesToStart := []int{}
				for _, message := range startMessages() {
					indexesToStart = append(indexesToStart, message.IndexToStart)
				}
				Expect(indexesToStart).To(ContainElement(0))
				Expect(indexesToStart).To(ContainElement(1))
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
			otherApp = dea.GetApp(1)
			undesiredApp = dea.GetApp(2)
			yetAnotherApp = dea.GetApp(3)
			undesiredApp.AppGuid = app.AppGuid

			desiredStateData := make(map[string]models.DesiredAppState)
			desiredState := app.DesiredState(1)
			desiredStateData[desiredState.StoreKey()] = desiredState
			desiredState = otherApp.DesiredState(3)
			desiredStateData[desiredState.StoreKey()] = desiredState
			desiredState = yetAnotherApp.DesiredState(2)
			desiredStateData[desiredState.StoreKey()] = desiredState

			writeToQueue(desiredStateData)

			store.SyncHeartbeats(dea.HeartbeatWith(
				app.InstanceAtIndex(0).Heartbeat(),
				app.InstanceAtIndex(1).Heartbeat(),
				undesiredApp.InstanceAtIndex(0).Heartbeat(),
				otherApp.InstanceAtIndex(0).Heartbeat(),
				otherApp.InstanceAtIndex(2).Heartbeat(),
			))
		})

		AfterEach(func() {
			Eventually(done).Should(BeClosed())
		})

		It("should analyze each app-version combination separately", func() {
			_, _, _, err := analyzer.Analyze(appQueue)
			Expect(err).ToNot(HaveOccurred())

			Expect(startMessages()).To(HaveLen(3))

			expectedStartMessage := models.NewPendingStartMessage(clock.Now(), conf.GracePeriod(), 0, otherApp.AppGuid, otherApp.AppVersion, 1, 1.0/3.0, models.PendingStartMessageReasonMissing)
			Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedStartMessage)))

			expectedStartMessage = models.NewPendingStartMessage(clock.Now(), conf.GracePeriod(), 0, yetAnotherApp.AppGuid, yetAnotherApp.AppVersion, 0, 1.0, models.PendingStartMessageReasonMissing)
			Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedStartMessage)))

			expectedStartMessage = models.NewPendingStartMessage(clock.Now(), conf.GracePeriod(), 0, yetAnotherApp.AppGuid, yetAnotherApp.AppVersion, 1, 1.0, models.PendingStartMessageReasonMissing)
			Expect(startMessages()).To(ContainElement(EqualPendingStartMessage(expectedStartMessage)))

			Expect(stopMessages()).To(HaveLen(2))

			expectedStopMessage := models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, app.InstanceAtIndex(1).InstanceGuid, models.PendingStopMessageReasonExtra)
			Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedStopMessage)))

			expectedStopMessage = models.NewPendingStopMessage(clock.Now(), 0, conf.GracePeriod(), undesiredApp.AppGuid, undesiredApp.AppVersion, undesiredApp.InstanceAtIndex(0).InstanceGuid, models.PendingStopMessageReasonExtra)
			Expect(stopMessages()).To(ContainElement(EqualPendingStopMessage(expectedStopMessage)))
		})
	})

	Context("When the store is not fresh and/or fails to fetch data", func() {
		BeforeEach(func() {
			storeAdapter.Reset()

			desiredState := app.DesiredState(1)
			//this setup would, ordinarily, trigger a start and a stop
			desiredStateData := make(map[string]models.DesiredAppState)
			desiredStateData[desiredState.StoreKey()] = desiredState
			writeToQueue(desiredStateData)

			store.SyncHeartbeats(
				appfixture.NewAppFixture().Heartbeat(0),
			)
		})

		AfterEach(func() {
			Eventually(done).Should(BeClosed())
		})

		Context("when the actual state is not fresh", func() {
			It("should not send any start or stop messages", func() {
				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).To(Equal(storepackage.ActualIsNotFreshError))
				Expect(startMessages()).To(BeEmpty())
				Expect(stopMessages()).To(BeEmpty())
			})
		})

		Context("when the apps fail to fetch", func() {
			BeforeEach(func() {
				store.BumpActualFreshness(time.Unix(10, 0))
				storeAdapter.ListErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("apps", errors.New("oops!"))
			})

			It("should return the store's error and not send any start/stop messages", func() {
				_, _, _, err := analyzer.Analyze(appQueue)
				Expect(err).To(Equal(errors.New("oops!")))
				Expect(startMessages()).To(BeEmpty())
				Expect(stopMessages()).To(BeEmpty())
			})
		})
	})

	Context("when the desired app queue fetcher encounters an returns an error", func() {
		BeforeEach(func() {
			appQueue.SetFetchDesiredAppsSuccess(false)

			desired := app.DesiredState(3)
			desired.State = models.AppStateStarted
			desiredStateData := make(map[string]models.DesiredAppState)
			desiredStateData[desired.StoreKey()] = desired

			writeToQueue(desiredStateData)

			store.SyncHeartbeats(app.Heartbeat(3))
		})

		AfterEach(func() {
			Eventually(done).Should(BeClosed())
		})

		It("does not send any app update", func() {
			_, _, _, err := analyzer.Analyze(appQueue)
			Expect(err).To(HaveOccurred())
			Expect(startMessages()).To(BeEmpty())
			Expect(stopMessages()).To(BeEmpty())
		})
	})
})
