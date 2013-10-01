package sender_test

import (
	"errors"
	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/sender"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestore"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Sender", func() {
	var (
		store        *fakestore.FakeStore
		sender       *Sender
		messageBus   *fake_cfmessagebus.FakeMessageBus
		timeProvider *faketimeprovider.FakeTimeProvider
		app1         app.App
	)

	BeforeEach(func() {
		store = fakestore.NewFakeStore()
		messageBus = fake_cfmessagebus.NewFakeMessageBus()
		timeProvider = &faketimeprovider.FakeTimeProvider{}
		app1 = app.NewApp()
		conf, _ := config.DefaultConfig()

		sender = New(store, conf, messageBus, timeProvider, fakelogger.NewFakeLogger())
		store.BumpActualFreshness(time.Unix(10, 0))
		store.BumpDesiredFreshness(time.Unix(10, 0))
	})

	Context("when the sender fails to pull messages out of the start queue", func() {
		BeforeEach(func() {
			store.GetStartMessagesError = errors.New("oops")
		})

		It("should return an error and not send any messages", func() {
			err := sender.Send()
			Ω(err).Should(Equal(errors.New("oops")))
			Ω(messageBus.PublishedMessages).Should(BeEmpty())
		})
	})

	Context("when the sender fails to pull messages out of the stop queue", func() {
		BeforeEach(func() {
			store.GetStopMessagesError = errors.New("oops")
		})

		It("should return an error and not send any messages", func() {
			err := sender.Send()
			Ω(err).Should(Equal(errors.New("oops")))
			Ω(messageBus.PublishedMessages).Should(BeEmpty())
		})
	})

	Context("when the sender fails to fetch the actual state", func() {
		BeforeEach(func() {
			store.GetActualStateError = errors.New("oops")
		})

		It("should return an error and not send any messages", func() {
			err := sender.Send()
			Ω(err).Should(Equal(errors.New("oops")))
			Ω(messageBus.PublishedMessages).Should(BeEmpty())
		})
	})

	Context("when there are no start messages in the queue", func() {
		It("should not send any messages", func() {
			err := sender.Send()
			Ω(err).ShouldNot(HaveOccured())
			Ω(messageBus.PublishedMessages).Should(BeEmpty())
		})
	})

	Context("when there are no stop messages in the queue", func() {
		It("should not send any messages", func() {
			err := sender.Send()
			Ω(err).ShouldNot(HaveOccured())
			Ω(messageBus.PublishedMessages).Should(BeEmpty())
		})
	})

	Context("when there are start messages", func() {
		var keepAliveTime int
		var sentOn int64
		var err error

		JustBeforeEach(func() {
			store.SaveDesiredState([]models.DesiredAppState{app1.DesiredState(0)})

			message := models.NewQueueStartMessage(time.Unix(100, 0), 30, keepAliveTime, app1.AppGuid, app1.AppVersion, 0)
			message.SentOn = sentOn
			store.SaveQueueStartMessages([]models.QueueStartMessage{
				message,
			})

			err = sender.Send()
		})

		BeforeEach(func() {
			keepAliveTime = 0
			sentOn = 0
			err = nil
		})

		Context("and it is not time to send the message yet", func() {
			BeforeEach(func() {
				timeProvider.TimeToProvide = time.Unix(129, 0)
			})

			It("should not error", func() {
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should not send the messages", func() {
				Ω(messageBus.PublishedMessages).ShouldNot(HaveKey("hm9000.start"))
			})

			It("should leave the messages in the queue", func() {
				messages, _ := store.GetQueueStartMessages()
				Ω(messages).Should(HaveLen(1))
			})
		})

		Context("and it is time to send the message", func() {
			BeforeEach(func() {
				timeProvider.TimeToProvide = time.Unix(130, 0)
			})

			It("should send the message", func() {
				Ω(messageBus.PublishedMessages["hm9000.start"]).Should(HaveLen(1))
				message, _ := models.NewStartMessageFromJSON(messageBus.PublishedMessages["hm9000.start"][0])
				Ω(message).Should(Equal(models.StartMessage{
					AppGuid:        app1.AppGuid,
					AppVersion:     app1.AppVersion,
					InstanceIndex:  0,
					RunningIndices: models.RunningIndices{},
				}))
			})

			It("should not error", func() {
				Ω(err).ShouldNot(HaveOccured())
			})

			Context("when the message should be kept alive", func() {
				BeforeEach(func() {
					keepAliveTime = 30
				})

				It("should update the sent on times", func() {
					messages, _ := store.GetQueueStartMessages()
					Ω(messages[0].SentOn).Should(Equal(timeProvider.Time().Unix()))
				})
			})

			Context("when the KeepAlive = 0", func() {
				BeforeEach(func() {
					keepAliveTime = 0
				})

				It("should just delete the message after sending it", func() {
					messages, _ := store.GetQueueStartMessages()
					Ω(messages).Should(BeEmpty())
				})
			})

			Context("when the message fails to send", func() {
				BeforeEach(func() {
					messageBus.PublishError = errors.New("oops")
				})

				It("should return an error", func() {
					Ω(err).Should(Equal(errors.New("oops")))
				})
			})

			Context("when the queue update fails", func() {
				BeforeEach(func() {
					store.SaveStartMessagesError = errors.New("oops")
				})

				It("should return an error", func() {
					Ω(err).Should(Equal(errors.New("oops")))
				})
			})

			Context("when the delete fails", func() {
				BeforeEach(func() {
					store.DeleteStartMessagesError = errors.New("oops")
				})

				It("should return an error", func() {
					Ω(err).Should(Equal(errors.New("oops")))
				})
			})
		})

		Context("When the message has already been sent", func() {
			BeforeEach(func() {
				sentOn = 130
				keepAliveTime = 30
			})

			Context("and the keep alive has elapsed", func() {
				BeforeEach(func() {
					timeProvider.TimeToProvide = time.Unix(160, 0)
				})

				It("should delete the message and not send it", func() {
					messages, _ := store.GetQueueStartMessages()
					Ω(messages).Should(BeEmpty())
					Ω(messageBus.PublishedMessages).ShouldNot(HaveKey("hm9000.start"))
				})
			})

			Context("and the keep alive has not elapsed", func() {
				BeforeEach(func() {
					timeProvider.TimeToProvide = time.Unix(159, 0)
				})

				It("should neither delete the message nor send it", func() {
					messages, _ := store.GetQueueStartMessages()
					Ω(messages).Should(HaveLen(1))

					Ω(messageBus.PublishedMessages).ShouldNot(HaveKey("hm9000.start"))
				})
			})
		})
	})

	Context("when there are stop messages", func() {
		var keepAliveTime int
		var sentOn int64
		var err error

		JustBeforeEach(func() {
			store.SaveActualState([]models.InstanceHeartbeat{
				app1.GetInstance(0).Heartbeat(0),
				app1.GetInstance(1).Heartbeat(0),
			})

			message := models.NewQueueStopMessage(time.Unix(100, 0), 30, keepAliveTime, app1.GetInstance(0).InstanceGuid)
			message.SentOn = sentOn
			store.SaveQueueStopMessages([]models.QueueStopMessage{
				message,
			})

			err = sender.Send()
		})

		BeforeEach(func() {
			keepAliveTime = 0
			sentOn = 0
			err = nil
		})

		Context("and it is not time to send the message yet", func() {
			BeforeEach(func() {
				timeProvider.TimeToProvide = time.Unix(129, 0)
			})

			It("should not error", func() {
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should not send the messages", func() {
				Ω(messageBus.PublishedMessages).ShouldNot(HaveKey("hm9000.stop"))
			})

			It("should leave the messages in the queue", func() {
				messages, _ := store.GetQueueStopMessages()
				Ω(messages).Should(HaveLen(1))
			})
		})

		Context("and it is time to send the message", func() {
			BeforeEach(func() {
				timeProvider.TimeToProvide = time.Unix(130, 0)
			})

			It("should not error", func() {
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should send the message", func() {
				Ω(messageBus.PublishedMessages["hm9000.stop"]).Should(HaveLen(1))
				message, _ := models.NewStopMessageFromJSON(messageBus.PublishedMessages["hm9000.stop"][0])
				Ω(message).Should(Equal(models.StopMessage{
					AppGuid:        app1.AppGuid,
					AppVersion:     app1.AppVersion,
					InstanceIndex:  0,
					InstanceGuid:   app1.GetInstance(0).InstanceGuid,
					RunningIndices: models.RunningIndices{"0": 1, "1": 1},
				}))
			})

			Context("when the message should be kept alive", func() {
				BeforeEach(func() {
					keepAliveTime = 30
				})

				It("should update the sent on times", func() {
					messages, _ := store.GetQueueStopMessages()
					Ω(messages[0].SentOn).Should(Equal(timeProvider.Time().Unix()))
				})
			})

			Context("when the KeepAlive = 0", func() {
				BeforeEach(func() {
					keepAliveTime = 0
				})

				It("should just delete the message after sending it", func() {
					messages, _ := store.GetQueueStopMessages()
					Ω(messages).Should(BeEmpty())
				})
			})

			Context("when the message fails to send", func() {
				BeforeEach(func() {
					messageBus.PublishError = errors.New("oops")
				})

				It("should return an error", func() {
					Ω(err).Should(Equal(errors.New("oops")))
				})
			})

			Context("when the queue update fails", func() {
				BeforeEach(func() {
					store.SaveStopMessagesError = errors.New("oops")
				})

				It("should return an error", func() {
					Ω(err).Should(Equal(errors.New("oops")))
				})
			})

			Context("when the delete fails", func() {
				BeforeEach(func() {
					store.DeleteStopMessagesError = errors.New("oops")
				})

				It("should return an error", func() {
					Ω(err).Should(Equal(errors.New("oops")))
				})
			})

		})

		Context("When the message has already been sent", func() {
			BeforeEach(func() {
				sentOn = 130
				keepAliveTime = 30
			})

			It("should not error", func() {
				Ω(err).ShouldNot(HaveOccured())
			})

			Context("and the keep alive has elapsed", func() {
				BeforeEach(func() {
					timeProvider.TimeToProvide = time.Unix(160, 0)
				})

				It("should delete the message and not send it", func() {
					messages, _ := store.GetQueueStopMessages()
					Ω(messages).Should(BeEmpty())
					Ω(messageBus.PublishedMessages).ShouldNot(HaveKey("hm9000.stop"))
				})
			})

			Context("and the keep alive has not elapsed", func() {
				BeforeEach(func() {
					timeProvider.TimeToProvide = time.Unix(159, 0)
				})

				It("should neither delete the message nor send it", func() {
					messages, _ := store.GetQueueStopMessages()
					Ω(messages).Should(HaveLen(1))

					Ω(messageBus.PublishedMessages).ShouldNot(HaveKey("hm9000.stop"))
				})
			})
		})
	})

	Describe("Verifying that start messages should be sent", func() {
		var err error
		var indexToStart int

		JustBeforeEach(func() {
			timeProvider.TimeToProvide = time.Unix(130, 0)
			message := models.NewQueueStartMessage(time.Unix(100, 0), 30, 10, app1.AppGuid, app1.AppVersion, indexToStart)
			message.SentOn = 0
			store.SaveQueueStartMessages([]models.QueueStartMessage{
				message,
			})

			err = sender.Send()
		})

		BeforeEach(func() {
			err = nil
			indexToStart = 0
		})

		assertMessageWasNotSent := func() {
			It("should ignore the keep-alive and delete the start message from queue", func() {
				messages, _ := store.GetQueueStartMessages()
				Ω(messages).Should(HaveLen(0))
			})

			It("should not send the start message", func() {
				Ω(messageBus.PublishedMessages).ShouldNot(HaveKey("hm9000.start"))
			})
		}

		assertMessageWasSent := func(expectedRunningIndices models.RunningIndices) {
			It("should honor the keep alive of the start message", func() {
				messages, _ := store.GetQueueStartMessages()
				Ω(messages).Should(HaveLen(1))
				Ω(messages[0].SentOn).Should(BeNumerically("==", 130))
			})

			It("should send the start message", func() {
				Ω(messageBus.PublishedMessages["hm9000.start"]).Should(HaveLen(1))
				message, _ := models.NewStartMessageFromJSON(messageBus.PublishedMessages["hm9000.start"][0])
				Ω(message).Should(Equal(models.StartMessage{
					AppGuid:        app1.AppGuid,
					AppVersion:     app1.AppVersion,
					InstanceIndex:  0,
					RunningIndices: expectedRunningIndices,
				}))
			})
		}

		Context("When the app is still desired", func() {
			BeforeEach(func() {
				store.SaveDesiredState([]models.DesiredAppState{app1.DesiredState(0)})
			})

			Context("when the index-to-start is within the # of desired instances", func() {
				BeforeEach(func() {
					indexToStart = 0
				})

				Context("when there are no running instances at all for that app", func() {
					assertMessageWasSent(models.RunningIndices{})
				})

				Context("when there is no running instance reporting at that index", func() {
					BeforeEach(func() {
						store.SaveActualState([]models.InstanceHeartbeat{
							app1.GetInstance(1).Heartbeat(0),
							app1.GetInstance(2).Heartbeat(0),
						})
					})
					assertMessageWasSent(models.RunningIndices{"1": 1, "2": 1})
				})

				Context("when there *is* a running instance reporting at that index", func() {
					BeforeEach(func() {
						store.SaveActualState([]models.InstanceHeartbeat{
							app1.GetInstance(0).Heartbeat(0),
						})
					})

					assertMessageWasNotSent()
				})
			})

			Context("when the index-to-start is beyond the # of desired instances", func() {
				BeforeEach(func() {
					indexToStart = 1
				})

				assertMessageWasNotSent()
			})
		})

		Context("When the app is no longer desired", func() {
			assertMessageWasNotSent()
		})
	})

	Describe("Verifying that stop messages should be sent", func() {
		var err error
		var indexToStop int

		JustBeforeEach(func() {
			timeProvider.TimeToProvide = time.Unix(130, 0)
			message := models.NewQueueStopMessage(time.Unix(100, 0), 30, 10, app1.GetInstance(indexToStop).InstanceGuid)
			message.SentOn = 0
			store.SaveQueueStopMessages([]models.QueueStopMessage{
				message,
			})

			err = sender.Send()
		})

		BeforeEach(func() {
			indexToStop = 0
		})

		assertMessageWasNotSent := func() {
			It("should ignore the keep-alive and delete the stop message from queue", func() {
				messages, _ := store.GetQueueStopMessages()
				Ω(messages).Should(HaveLen(0))
			})

			It("should not send the stop message", func() {
				Ω(messageBus.PublishedMessages).ShouldNot(HaveKey("hm9000.stop"))
			})
		}

		assertMessageWasSent := func(indexToStop int, expectedRunningIndices models.RunningIndices) {
			It("should honor the keep alive of the stop message", func() {
				messages, _ := store.GetQueueStopMessages()
				Ω(messages).Should(HaveLen(1))
				Ω(messages[0].SentOn).Should(BeNumerically("==", 130))
			})

			It("should send the stop message", func() {
				Ω(messageBus.PublishedMessages["hm9000.stop"]).Should(HaveLen(1))
				message, _ := models.NewStopMessageFromJSON(messageBus.PublishedMessages["hm9000.stop"][0])
				Ω(message).Should(Equal(models.StopMessage{
					AppGuid:        app1.AppGuid,
					AppVersion:     app1.AppVersion,
					InstanceIndex:  indexToStop,
					InstanceGuid:   app1.GetInstance(indexToStop).InstanceGuid,
					RunningIndices: expectedRunningIndices,
				}))
			})
		}

		Context("When the app is still desired", func() {
			BeforeEach(func() {
				store.SaveDesiredState([]models.DesiredAppState{app1.DesiredState(0)})
			})

			Context("When index is still running", func() {
				BeforeEach(func() {
					store.SaveActualState([]models.InstanceHeartbeat{
						app1.GetInstance(0).Heartbeat(0),
						app1.GetInstance(1).Heartbeat(0),
					})
				})

				Context("When index-to-stop is within the number of desired instances", func() {
					BeforeEach(func() {
						indexToStop = 0
					})

					Context("When there are other running instances on the index", func() {
						BeforeEach(func() {
							instance := app1.GetInstance(0)
							instance.InstanceGuid = app.Guid()

							store.SaveActualState([]models.InstanceHeartbeat{
								instance.Heartbeat(0),
							})
						})

						assertMessageWasSent(0, models.RunningIndices{"0": 2, "1": 1})
					})

					Context("When there are no other running instances on the index", func() {
						assertMessageWasNotSent()
					})
				})

				Context("When index-to-stop is beyond the number of desired instances", func() {
					BeforeEach(func() {
						indexToStop = 1
					})

					assertMessageWasSent(1, models.RunningIndices{"0": 1, "1": 1})
				})
			})

			Context("When index is not running", func() {
				assertMessageWasNotSent()
			})
		})

		Context("When the app is no longer desired", func() {
			Context("when the instance is still running", func() {
				BeforeEach(func() {
					store.SaveActualState([]models.InstanceHeartbeat{
						app1.GetInstance(0).Heartbeat(0),
						app1.GetInstance(1).Heartbeat(0),
					})
				})
				assertMessageWasSent(0, models.RunningIndices{"0": 1, "1": 1})
			})

			Context("when the instance is not running", func() {
				assertMessageWasNotSent()
			})
		})
	})
})
