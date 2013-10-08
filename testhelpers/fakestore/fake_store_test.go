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
			desired1 := app1.DesiredState()
			desired2 := app2.DesiredState()

			err := store.SaveDesiredState([]models.DesiredAppState{desired1, desired1, desired2})
			Ω(err).ShouldNot(HaveOccured())

			desired, err := store.GetDesiredState()
			Ω(err).ShouldNot(HaveOccured())
			Ω(desired).Should(HaveLen(2))
			Ω(desired).Should(ContainElement(EqualDesiredState(desired1)))
			Ω(desired).Should(ContainElement(EqualDesiredState(desired2)))

			desired2.NumberOfInstances = 17
			desired3 := app.NewApp().DesiredState()

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
			desired1 := app1.DesiredState()
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
			heartbeat1 := app1.InstanceAtIndex(0).Heartbeat()
			heartbeat2 := app2.InstanceAtIndex(0).Heartbeat()

			err := store.SaveActualState([]models.InstanceHeartbeat{heartbeat1, heartbeat1, heartbeat2})
			Ω(err).ShouldNot(HaveOccured())

			actual, err := store.GetActualState()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(2))
			Ω(actual).Should(ContainElement(heartbeat1))
			Ω(actual).Should(ContainElement(heartbeat2))

			heartbeat2.State = models.InstanceStateCrashed
			heartbeat3 := app1.InstanceAtIndex(1).Heartbeat()

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
			heartbeat1 := app1.InstanceAtIndex(0).Heartbeat()
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
			message1 := models.NewPendingStartMessage(time.Unix(100, 0), 10, 4, "ABC", "123", 1, 1.0)
			message2 := models.NewPendingStartMessage(time.Unix(100, 0), 10, 4, "ABC", "456", 1, 1.0)

			err := store.SavePendingStartMessages([]models.PendingStartMessage{message1, message1, message2})
			Ω(err).ShouldNot(HaveOccured())

			actual, err := store.GetPendingStartMessages()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(2))
			Ω(actual).Should(ContainElement(message1))
			Ω(actual).Should(ContainElement(message2))

			message2.SendOn = 120
			message3 := models.NewPendingStartMessage(time.Unix(100, 0), 10, 4, "DEF", "123", 1, 1.0)

			err = store.SavePendingStartMessages([]models.PendingStartMessage{message2, message3})
			Ω(err).ShouldNot(HaveOccured())

			actual, err = store.GetPendingStartMessages()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(3))
			Ω(actual).Should(ContainElement(message1))
			Ω(actual).Should(ContainElement(message2))
			Ω(actual).Should(ContainElement(message3))

			err = store.DeletePendingStartMessages([]models.PendingStartMessage{message2, message3})
			Ω(err).ShouldNot(HaveOccured())
			actual, err = store.GetPendingStartMessages()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(1))
			Ω(actual).Should(ContainElement(message1))

			err = store.DeletePendingStartMessages([]models.PendingStartMessage{message2})
			Ω(err).Should(HaveOccured())
			Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

			store.Reset()

			actual, err = store.GetPendingStartMessages()
			Ω(actual).Should(BeEmpty())
			Ω(err).ShouldNot(HaveOccured())
		})

		It("should support returning errors", func() {
			message1 := models.NewPendingStartMessage(time.Unix(100, 0), 10, 4, "ABC", "123", 1, 1.0)
			store.SavePendingStartMessages([]models.PendingStartMessage{message1})

			errIn := errors.New("foo")

			store.SaveStartMessagesError = errIn
			err := store.SavePendingStartMessages([]models.PendingStartMessage{message1})
			Ω(err).Should(Equal(errIn))

			store.GetStartMessagesError = errIn
			desired, err := store.GetPendingStartMessages()
			Ω(desired).Should(BeEmpty())
			Ω(err).Should(Equal(errIn))

			store.Reset()
			err = store.SavePendingStartMessages([]models.PendingStartMessage{message1})
			Ω(err).ShouldNot(HaveOccured())
			_, err = store.GetPendingStartMessages()
			Ω(err).ShouldNot(HaveOccured())
		})
	})

	Describe("Setting, getting, and deleting stop messages", func() {
		It("should set, get, and delete the stop messages state", func() {
			message1 := models.NewPendingStopMessage(time.Unix(100, 0), 10, 4, "ABC")
			message2 := models.NewPendingStopMessage(time.Unix(100, 0), 10, 4, "DEF")

			err := store.SavePendingStopMessages([]models.PendingStopMessage{message1, message1, message2})
			Ω(err).ShouldNot(HaveOccured())

			actual, err := store.GetPendingStopMessages()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(2))
			Ω(actual).Should(ContainElement(message1))
			Ω(actual).Should(ContainElement(message2))

			message2.SendOn = 12310
			message3 := models.NewPendingStopMessage(time.Unix(100, 0), 10, 4, "GHI")

			err = store.SavePendingStopMessages([]models.PendingStopMessage{message2, message3})
			Ω(err).ShouldNot(HaveOccured())

			actual, err = store.GetPendingStopMessages()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(3))
			Ω(actual).Should(ContainElement(message1))
			Ω(actual).Should(ContainElement(message2))
			Ω(actual).Should(ContainElement(message3))

			err = store.DeletePendingStopMessages([]models.PendingStopMessage{message2, message3})
			Ω(err).ShouldNot(HaveOccured())
			actual, err = store.GetPendingStopMessages()
			Ω(err).ShouldNot(HaveOccured())
			Ω(actual).Should(HaveLen(1))
			Ω(actual).Should(ContainElement(message1))

			err = store.DeletePendingStopMessages([]models.PendingStopMessage{message2})
			Ω(err).Should(HaveOccured())
			Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

			store.Reset()

			actual, err = store.GetPendingStopMessages()
			Ω(actual).Should(BeEmpty())
			Ω(err).ShouldNot(HaveOccured())
		})

		It("should support returning errors", func() {
			message1 := models.NewPendingStopMessage(time.Unix(100, 0), 10, 4, "ABC")
			store.SavePendingStopMessages([]models.PendingStopMessage{message1})

			errIn := errors.New("foo")

			store.SaveStopMessagesError = errIn
			err := store.SavePendingStopMessages([]models.PendingStopMessage{message1})
			Ω(err).Should(Equal(errIn))

			store.GetStopMessagesError = errIn
			desired, err := store.GetPendingStopMessages()
			Ω(desired).Should(BeEmpty())
			Ω(err).Should(Equal(errIn))

			store.Reset()
			err = store.SavePendingStopMessages([]models.PendingStopMessage{message1})
			Ω(err).ShouldNot(HaveOccured())
			_, err = store.GetPendingStopMessages()
			Ω(err).ShouldNot(HaveOccured())
		})
	})

	Describe("Setting, getting, and deleting crash counts", func() {
		It("should set, get, and delete the crash counts", func() {
			crashCount1 := models.CrashCount{
				AppGuid:       models.Guid(),
				AppVersion:    models.Guid(),
				InstanceIndex: 1,
				CrashCount:    12,
			}

			crashCount2 := models.CrashCount{
				AppGuid:       models.Guid(),
				AppVersion:    models.Guid(),
				InstanceIndex: 2,
				CrashCount:    7,
			}

			err := store.SaveCrashCounts([]models.CrashCount{crashCount1, crashCount1, crashCount2})
			Ω(err).ShouldNot(HaveOccured())

			crashCounts, err := store.GetCrashCounts()
			Ω(err).ShouldNot(HaveOccured())
			Ω(crashCounts).Should(HaveLen(2))
			Ω(crashCounts).Should(ContainElement(crashCount1))
			Ω(crashCounts).Should(ContainElement(crashCount2))

			crashCount2.CrashCount = 8
			crashCount3 := models.CrashCount{
				AppGuid:       models.Guid(),
				AppVersion:    models.Guid(),
				InstanceIndex: 3,
				CrashCount:    9,
			}

			err = store.SaveCrashCounts([]models.CrashCount{crashCount2, crashCount3})
			Ω(err).ShouldNot(HaveOccured())

			crashCounts, err = store.GetCrashCounts()
			Ω(err).ShouldNot(HaveOccured())
			Ω(crashCounts).Should(HaveLen(3))
			Ω(crashCounts).Should(ContainElement(crashCount1))
			Ω(crashCounts).Should(ContainElement(crashCount2))
			Ω(crashCounts).Should(ContainElement(crashCount3))

			err = store.DeleteCrashCounts([]models.CrashCount{crashCount2, crashCount3})
			Ω(err).ShouldNot(HaveOccured())
			crashCounts, err = store.GetCrashCounts()
			Ω(err).ShouldNot(HaveOccured())
			Ω(crashCounts).Should(HaveLen(1))
			Ω(crashCounts).Should(ContainElement(crashCount1))

			err = store.DeleteCrashCounts([]models.CrashCount{crashCount2})
			Ω(err).Should(HaveOccured())
			Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

			store.Reset()

			crashCounts, err = store.GetCrashCounts()
			Ω(crashCounts).Should(BeEmpty())
			Ω(err).ShouldNot(HaveOccured())
		})

		It("should support returning errors", func() {
			crashCount1 := models.CrashCount{
				AppGuid:       models.Guid(),
				AppVersion:    models.Guid(),
				InstanceIndex: 1,
				CrashCount:    12,
			}
			store.SaveCrashCounts([]models.CrashCount{crashCount1})

			errIn := errors.New("foo")

			store.SaveCrashCountsError = errIn
			err := store.SaveCrashCounts([]models.CrashCount{crashCount1})
			Ω(err).Should(Equal(errIn))

			store.GetCrashCountsError = errIn
			crashCounts, err := store.GetCrashCounts()
			Ω(crashCounts).Should(BeEmpty())
			Ω(err).Should(Equal(errIn))

			store.Reset()
			err = store.SaveCrashCounts([]models.CrashCount{crashCount1})
			Ω(err).ShouldNot(HaveOccured())
			_, err = store.GetCrashCounts()
			Ω(err).ShouldNot(HaveOccured())
		})
	})
})
