package fakestore_test

import (
	"errors"
	. "github.com/cloudfoundry/hm9000/testhelpers/fakestore"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/models"
	storePackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	"time"
)

var _ = Describe("FakeStore", func() {
	var store *FakeStore
	var storeType storePackage.Store
	var app1 app.App
	var app2 app.App

	BeforeEach(func() {
		store = NewFakeStore()
		storeType = NewFakeStore() //use compiler to verify that we satisfy the store interface
		app1 = app.NewApp()
		app2 = app.NewApp()
	})

	It("should start off empty", func() {
		desired, err := store.GetDesiredState()
		Ω(desired).Should(BeEmpty())
		Ω(err).ShouldNot(HaveOccured())

		actual, err := store.GetActualState()
		Ω(actual).Should(BeEmpty())
		Ω(err).ShouldNot(HaveOccured())

		Ω(store.ActualFreshnessTimestamp).Should(BeZero())
		Ω(store.DesiredFreshnessTimestamp).Should(BeZero())
		Ω(store.ActualFreshnessComparisonTimestamp).Should(BeZero())
	})

	Describe("freshness", func() {
		It("should support bumping freshness and save the passed in timestamp", func() {
			fresh, err := store.IsDesiredStateFresh()
			Ω(err).ShouldNot(HaveOccured())
			Ω(fresh).Should(BeFalse())

			fresh, err = store.IsActualStateFresh(time.Unix(10, 0))
			Ω(err).ShouldNot(HaveOccured())
			Ω(fresh).Should(BeFalse())
			Ω(store.ActualFreshnessComparisonTimestamp).Should(Equal(time.Unix(10, 0)))

			err = store.BumpDesiredFreshness(time.Unix(17, 0))
			Ω(store.DesiredFreshnessTimestamp.Equal(time.Unix(17, 0))).Should(BeTrue())
			Ω(err).ShouldNot(HaveOccured())

			fresh, err = store.IsDesiredStateFresh()
			Ω(err).ShouldNot(HaveOccured())
			Ω(fresh).Should(BeTrue())

			err = store.BumpActualFreshness(time.Unix(12, 0))
			Ω(store.ActualFreshnessTimestamp.Equal(time.Unix(12, 0))).Should(BeTrue())
			Ω(err).ShouldNot(HaveOccured())

			fresh, err = store.IsActualStateFresh(time.Unix(17, 0))
			Ω(err).ShouldNot(HaveOccured())
			Ω(fresh).Should(BeTrue())

			store.Reset()
			Ω(store.DesiredFreshnessTimestamp).Should(BeZero())
			Ω(store.ActualFreshnessTimestamp).Should(BeZero())
			Ω(store.ActualFreshnessComparisonTimestamp).Should(BeZero())
		})

		It("should support returning errors", func() {
			errIn := errors.New("foo")
			store.BumpDesiredFreshnessError = errIn
			err := store.BumpDesiredFreshness(time.Unix(17, 0))
			Ω(err).Should(Equal(errIn))

			store.BumpActualFreshnessError = errIn
			err = store.BumpActualFreshness(time.Unix(17, 0))
			Ω(err).Should(Equal(errIn))

			store.IsDesiredStateFreshError = errIn
			_, err = store.IsDesiredStateFresh()
			Ω(err).Should(Equal(errIn))

			store.IsActualStateFreshError = errIn
			_, err = store.IsActualStateFresh(time.Unix(17, 0))
			Ω(err).Should(Equal(errIn))

			store.Reset()

			err = store.BumpDesiredFreshness(time.Unix(12, 0))
			Ω(err).ShouldNot(HaveOccured())

			err = store.BumpActualFreshness(time.Unix(12, 0))
			Ω(err).ShouldNot(HaveOccured())

			_, err = store.IsDesiredStateFresh()
			Ω(err).ShouldNot(HaveOccured())

			_, err = store.IsActualStateFresh(time.Unix(17, 0))
			Ω(err).ShouldNot(HaveOccured())
		})
	})

	Describe("Setting, getting, and deleting desired state", func() {
		It("should set, get, and delete the desired state", func() {
			desired1 := app1.DesiredState(0)
			desired2 := app2.DesiredState(0)

			err := store.SaveDesiredState([]models.DesiredAppState{desired1, desired1, desired2})
			Ω(err).ShouldNot(HaveOccured())

			desired, err := store.GetDesiredState()
			Ω(err).ShouldNot(HaveOccured())
			Ω(desired).Should(HaveLen(2))
			Ω(desired).Should(ContainElement(EqualDesiredState(desired1)))
			Ω(desired).Should(ContainElement(EqualDesiredState(desired2)))

			desired2.NumberOfInstances = 17
			desired3 := app.NewApp().DesiredState(2)

			err = store.SaveDesiredState([]models.DesiredAppState{desired2, desired3})
			Ω(err).ShouldNot(HaveOccured())

			desired, err = store.GetDesiredState()
			Ω(err).ShouldNot(HaveOccured())
			Ω(desired).Should(HaveLen(3))
			Ω(desired).Should(ContainElement(EqualDesiredState(desired1)))
			Ω(desired).Should(ContainElement(EqualDesiredState(desired2)))
			Ω(desired).Should(ContainElement(EqualDesiredState(desired3)))

			err = store.DeleteDesiredState([]models.DesiredAppState{desired2, desired3})
			Ω(err).ShouldNot(HaveOccured())
			desired, err = store.GetDesiredState()
			Ω(err).ShouldNot(HaveOccured())
			Ω(desired).Should(HaveLen(1))
			Ω(desired).Should(ContainElement(EqualDesiredState(desired1)))

			err = store.DeleteDesiredState([]models.DesiredAppState{desired2})
			Ω(err).Should(HaveOccured())
			Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

			store.Reset()

			desired, err = store.GetDesiredState()
			Ω(desired).Should(BeEmpty())
			Ω(err).ShouldNot(HaveOccured())
		})

		It("should support returning errors", func() {
			desired1 := app1.DesiredState(0)
			store.SaveDesiredState([]models.DesiredAppState{desired1})

			errIn := errors.New("foo")

			store.SaveDesiredStateError = errIn
			err := store.SaveDesiredState([]models.DesiredAppState{desired1})
			Ω(err).Should(Equal(errIn))

			store.GetDesiredStateError = errIn
			desired, err := store.GetDesiredState()
			Ω(desired).Should(BeEmpty())
			Ω(err).Should(Equal(errIn))

			store.Reset()
			err = store.SaveDesiredState([]models.DesiredAppState{desired1})
			Ω(err).ShouldNot(HaveOccured())
			_, err = store.GetDesiredState()
			Ω(err).ShouldNot(HaveOccured())
		})
	})

	Describe("Setting, getting, and deleting actual state", func() {
		It("should set, get, and delete the actual state", func() {
			heartbeat1 := app1.GetInstance(0).Heartbeat(12)
			heartbeat2 := app2.GetInstance(0).Heartbeat(10)

			err := store.SaveActualState([]models.InstanceHeartbeat{heartbeat1, heartbeat1, heartbeat2})
			Ω(err).ShouldNot(HaveOccured())

			actual, err := store.GetActualState()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(2))
			Ω(actual).Should(ContainElement(heartbeat1))
			Ω(actual).Should(ContainElement(heartbeat2))

			heartbeat2.State = models.InstanceStateCrashed
			heartbeat3 := app1.GetInstance(1).Heartbeat(12)

			err = store.SaveActualState([]models.InstanceHeartbeat{heartbeat2, heartbeat3})
			Ω(err).ShouldNot(HaveOccured())

			actual, err = store.GetActualState()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(3))
			Ω(actual).Should(ContainElement(heartbeat1))
			Ω(actual).Should(ContainElement(heartbeat2))
			Ω(actual).Should(ContainElement(heartbeat3))

			err = store.DeleteActualState([]models.InstanceHeartbeat{heartbeat2, heartbeat3})
			Ω(err).ShouldNot(HaveOccured())
			actual, err = store.GetActualState()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(1))
			Ω(actual).Should(ContainElement(heartbeat1))

			err = store.DeleteActualState([]models.InstanceHeartbeat{heartbeat2})
			Ω(err).Should(HaveOccured())
			Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

			store.Reset()

			actual, err = store.GetActualState()
			Ω(actual).Should(BeEmpty())
			Ω(err).ShouldNot(HaveOccured())
		})

		It("should support returning errors", func() {
			heartbeat1 := app1.GetInstance(0).Heartbeat(12)
			store.SaveActualState([]models.InstanceHeartbeat{heartbeat1})

			errIn := errors.New("foo")

			store.SaveActualStateError = errIn
			err := store.SaveActualState([]models.InstanceHeartbeat{heartbeat1})
			Ω(err).Should(Equal(errIn))

			store.GetActualStateError = errIn
			desired, err := store.GetActualState()
			Ω(desired).Should(BeEmpty())
			Ω(err).Should(Equal(errIn))

			store.Reset()
			err = store.SaveActualState([]models.InstanceHeartbeat{heartbeat1})
			Ω(err).ShouldNot(HaveOccured())
			_, err = store.GetActualState()
			Ω(err).ShouldNot(HaveOccured())
		})
	})

	Describe("Setting, getting, and deleting start messages", func() {
		It("should set, get, and delete the start messages state", func() {
			message1 := models.NewQueueStartMessage(time.Unix(100, 0), 10, 4, "ABC", "123", 1, 1.0)
			message2 := models.NewQueueStartMessage(time.Unix(100, 0), 10, 4, "ABC", "456", 1, 1.0)

			err := store.SaveQueueStartMessages([]models.QueueStartMessage{message1, message1, message2})
			Ω(err).ShouldNot(HaveOccured())

			actual, err := store.GetQueueStartMessages()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(2))
			Ω(actual).Should(ContainElement(message1))
			Ω(actual).Should(ContainElement(message2))

			message2.SendOn = 120
			message3 := models.NewQueueStartMessage(time.Unix(100, 0), 10, 4, "DEF", "123", 1, 1.0)

			err = store.SaveQueueStartMessages([]models.QueueStartMessage{message2, message3})
			Ω(err).ShouldNot(HaveOccured())

			actual, err = store.GetQueueStartMessages()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(3))
			Ω(actual).Should(ContainElement(message1))
			Ω(actual).Should(ContainElement(message2))
			Ω(actual).Should(ContainElement(message3))

			err = store.DeleteQueueStartMessages([]models.QueueStartMessage{message2, message3})
			Ω(err).ShouldNot(HaveOccured())
			actual, err = store.GetQueueStartMessages()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(1))
			Ω(actual).Should(ContainElement(message1))

			err = store.DeleteQueueStartMessages([]models.QueueStartMessage{message2})
			Ω(err).Should(HaveOccured())
			Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

			store.Reset()

			actual, err = store.GetQueueStartMessages()
			Ω(actual).Should(BeEmpty())
			Ω(err).ShouldNot(HaveOccured())
		})

		It("should support returning errors", func() {
			message1 := models.NewQueueStartMessage(time.Unix(100, 0), 10, 4, "ABC", "123", 1, 1.0)
			store.SaveQueueStartMessages([]models.QueueStartMessage{message1})

			errIn := errors.New("foo")

			store.SaveStartMessagesError = errIn
			err := store.SaveQueueStartMessages([]models.QueueStartMessage{message1})
			Ω(err).Should(Equal(errIn))

			store.GetStartMessagesError = errIn
			desired, err := store.GetQueueStartMessages()
			Ω(desired).Should(BeEmpty())
			Ω(err).Should(Equal(errIn))

			store.Reset()
			err = store.SaveQueueStartMessages([]models.QueueStartMessage{message1})
			Ω(err).ShouldNot(HaveOccured())
			_, err = store.GetQueueStartMessages()
			Ω(err).ShouldNot(HaveOccured())
		})
	})

	Describe("Setting, getting, and deleting stop messages", func() {
		It("should set, get, and delete the stop messages state", func() {
			message1 := models.NewQueueStopMessage(time.Unix(100, 0), 10, 4, "ABC")
			message2 := models.NewQueueStopMessage(time.Unix(100, 0), 10, 4, "DEF")

			err := store.SaveQueueStopMessages([]models.QueueStopMessage{message1, message1, message2})
			Ω(err).ShouldNot(HaveOccured())

			actual, err := store.GetQueueStopMessages()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(2))
			Ω(actual).Should(ContainElement(message1))
			Ω(actual).Should(ContainElement(message2))

			message2.SendOn = 12310
			message3 := models.NewQueueStopMessage(time.Unix(100, 0), 10, 4, "GHI")

			err = store.SaveQueueStopMessages([]models.QueueStopMessage{message2, message3})
			Ω(err).ShouldNot(HaveOccured())

			actual, err = store.GetQueueStopMessages()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(3))
			Ω(actual).Should(ContainElement(message1))
			Ω(actual).Should(ContainElement(message2))
			Ω(actual).Should(ContainElement(message3))

			err = store.DeleteQueueStopMessages([]models.QueueStopMessage{message2, message3})
			Ω(err).ShouldNot(HaveOccured())
			actual, err = store.GetQueueStopMessages()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(1))
			Ω(actual).Should(ContainElement(message1))

			err = store.DeleteQueueStopMessages([]models.QueueStopMessage{message2})
			Ω(err).Should(HaveOccured())
			Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

			store.Reset()

			actual, err = store.GetQueueStopMessages()
			Ω(actual).Should(BeEmpty())
			Ω(err).ShouldNot(HaveOccured())
		})

		It("should support returning errors", func() {
			message1 := models.NewQueueStopMessage(time.Unix(100, 0), 10, 4, "ABC")
			store.SaveQueueStopMessages([]models.QueueStopMessage{message1})

			errIn := errors.New("foo")

			store.SaveStopMessagesError = errIn
			err := store.SaveQueueStopMessages([]models.QueueStopMessage{message1})
			Ω(err).Should(Equal(errIn))

			store.GetStopMessagesError = errIn
			desired, err := store.GetQueueStopMessages()
			Ω(desired).Should(BeEmpty())
			Ω(err).Should(Equal(errIn))

			store.Reset()
			err = store.SaveQueueStopMessages([]models.QueueStopMessage{message1})
			Ω(err).ShouldNot(HaveOccured())
			_, err = store.GetQueueStopMessages()
			Ω(err).ShouldNot(HaveOccured())
		})
	})
})
