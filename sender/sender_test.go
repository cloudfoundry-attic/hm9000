package sender_test

import (
	"errors"
	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/sender"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
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

		sender = New(store, messageBus, timeProvider)
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

	//More tests for more cases (especially for running indices)
})
