package outbox_test

import (
	"errors"
	. "github.com/cloudfoundry/hm9000/helpers/outbox"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestore"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Outbox", func() {
	var (
		store         *fakestore.FakeStore
		logger        *fakelogger.FakeLogger
		startMessages []models.QueueStartMessage
		stopMessages  []models.QueueStopMessage

		outbox Outbox
	)

	BeforeEach(func() {
		store = fakestore.NewFakeStore()
		logger = fakelogger.NewFakeLogger()
		startMessages = []models.QueueStartMessage{
			models.NewQueueStartMessage(time.Unix(100, 0), 10, 4, "ABC", "123", 1, 1.0),
			models.NewQueueStartMessage(time.Unix(100, 0), 10, 4, "DEF", "123", 1, 1.0),
			models.NewQueueStartMessage(time.Unix(100, 0), 10, 4, "ABC", "123", 2, 1.0),
		}

		stopMessages = []models.QueueStopMessage{
			models.NewQueueStopMessage(time.Unix(100, 0), 10, 4, "ABC"),
			models.NewQueueStopMessage(time.Unix(100, 0), 10, 4, "DEF"),
			models.NewQueueStopMessage(time.Unix(100, 0), 10, 4, "GHI"),
		}

		outbox = New(store, logger)
	})

	Describe("Enqueuing", func() {
		var err error
		JustBeforeEach(func() {
			err = outbox.Enqueue(startMessages, stopMessages)
		})

		Context("When the store has no colliding messages", func() {
			It("should not error", func() {
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should store all the messages", func() {
				starts, _ := store.GetQueueStartMessages()
				stops, _ := store.GetQueueStopMessages()

				Ω(starts).Should(HaveLen(3))
				Ω(starts).Should(ContainElement(startMessages[0]))
				Ω(starts).Should(ContainElement(startMessages[1]))
				Ω(starts).Should(ContainElement(startMessages[2]))

				Ω(stops).Should(HaveLen(3))
				Ω(stops).Should(ContainElement(stopMessages[0]))
				Ω(stops).Should(ContainElement(stopMessages[1]))
				Ω(stops).Should(ContainElement(stopMessages[2]))
			})

			It("should log about each of the messages it is enqueueing", func() {
				Ω(logger.LoggedSubjects).Should(HaveLen(6))
				for i := 0; i < 3; i++ {
					Ω(logger.LoggedSubjects[i]).Should(Equal("Enqueuing Start Message"))
					Ω(logger.LoggedMessages[i][0]).Should(Equal(startMessages[i].LogDescription()))
				}
				for i := 0; i < 3; i++ {
					Ω(logger.LoggedSubjects[i+3]).Should(Equal("Enqueuing Stop Message"))
					Ω(logger.LoggedMessages[i+3][0]).Should(Equal(stopMessages[i].LogDescription()))
				}
			})
		})

		Context("when the store has colliding start messages", func() {
			var collidingStartMessage models.QueueStartMessage

			BeforeEach(func() {
				collidingStartMessage = models.NewQueueStartMessage(time.Unix(120, 0), 10, 4, "DEF", "123", 1, 1.0)
				store.SaveQueueStartMessages([]models.QueueStartMessage{
					collidingStartMessage,
				})
			})

			It("should not error", func() {
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should store the non-colliding messages, but leave the colliding message intact", func() {
				starts, _ := store.GetQueueStartMessages()
				stops, _ := store.GetQueueStopMessages()

				Ω(starts).Should(HaveLen(3))
				Ω(starts).Should(ContainElement(startMessages[0]))
				Ω(starts).ShouldNot(ContainElement(startMessages[1]))
				Ω(starts).Should(ContainElement(collidingStartMessage))
				Ω(starts).Should(ContainElement(startMessages[2]))

				Ω(stops).Should(HaveLen(3))
				Ω(stops).Should(ContainElement(stopMessages[0]))
				Ω(stops).Should(ContainElement(stopMessages[1]))
				Ω(stops).Should(ContainElement(stopMessages[2]))
			})

			It("should note that it did not enqueue the colliding message", func() {
				Ω(logger.LoggedSubjects).Should(HaveLen(6))
				Ω(logger.LoggedSubjects[1]).Should(Equal("Skipping Already Enqueued Start Message"))
				Ω(logger.LoggedMessages[1][0]).Should(Equal(startMessages[1].LogDescription()))
			})
		})

		Context("When the store has colliding stop messages", func() {
			var collidingStopMessage models.QueueStopMessage

			BeforeEach(func() {
				collidingStopMessage = models.NewQueueStopMessage(time.Unix(120, 0), 10, 4, "DEF")
				store.SaveQueueStopMessages([]models.QueueStopMessage{
					collidingStopMessage,
				})
			})

			It("should not error", func() {
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should store the non-colliding messages, but leave the colliding message intact", func() {
				starts, _ := store.GetQueueStartMessages()
				stops, _ := store.GetQueueStopMessages()

				Ω(starts).Should(HaveLen(3))
				Ω(starts).Should(ContainElement(startMessages[0]))
				Ω(starts).Should(ContainElement(startMessages[1]))
				Ω(starts).Should(ContainElement(startMessages[2]))

				Ω(stops).Should(HaveLen(3))
				Ω(stops).Should(ContainElement(stopMessages[0]))
				Ω(stops).ShouldNot(ContainElement(stopMessages[1]))
				Ω(stops).Should(ContainElement(collidingStopMessage))
				Ω(stops).Should(ContainElement(stopMessages[2]))
			})

			It("should note that it did not enqueue the colliding message", func() {
				Ω(logger.LoggedSubjects).Should(HaveLen(6))
				Ω(logger.LoggedSubjects[4]).Should(Equal("Skipping Already Enqueued Stop Message"))
				Ω(logger.LoggedMessages[4][0]).Should(Equal(stopMessages[1].LogDescription()))
			})
		})

		Context("When the store fails to fetch the currently enqueued start messages", func() {
			BeforeEach(func() {
				store.GetStartMessagesError = errors.New("oops")
			})

			It("should error", func() {
				Ω(err).Should(Equal(errors.New("oops")))
			})

			It("should not enqueue any messages", func() {
				store.GetStartMessagesError = nil
				starts, _ := store.GetQueueStartMessages()
				stops, _ := store.GetQueueStopMessages()
				Ω(starts).Should(BeEmpty())
				Ω(stops).Should(BeEmpty())
			})
		})

		Context("When the store fails to fetch the currently enqueued stop messages", func() {
			BeforeEach(func() {
				store.GetStopMessagesError = errors.New("oops")
			})

			It("should error", func() {
				Ω(err).Should(Equal(errors.New("oops")))
			})

			It("should not enqueue any messages", func() {
				store.GetStartMessagesError = nil
				starts, _ := store.GetQueueStartMessages()
				stops, _ := store.GetQueueStopMessages()
				Ω(starts).Should(BeEmpty())
				Ω(stops).Should(BeEmpty())
			})
		})

		Context("When the store fails to save the start messages", func() {
			BeforeEach(func() {
				store.SaveStartMessagesError = errors.New("oops")
			})

			It("should error", func() {
				Ω(err).Should(Equal(errors.New("oops")))
			})
		})

		Context("When the store fails to save the stop messages", func() {
			BeforeEach(func() {
				store.SaveStopMessagesError = errors.New("oops")
			})

			It("should error", func() {
				Ω(err).Should(Equal(errors.New("oops")))
			})
		})
	})
})
