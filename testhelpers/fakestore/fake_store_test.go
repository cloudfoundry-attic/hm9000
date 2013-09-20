package fakestore_test

import (
	"errors"
	. "github.com/cloudfoundry/hm9000/testhelpers/fakestore"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	"time"
)

var _ = Describe("FakeStore", func() {
	var store *FakeStore
	var app1 app.App
	var app2 app.App

	BeforeEach(func() {
		store = NewFakeStore()
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

		Ω(store.DesiredIsFresh).Should(BeFalse())
		Ω(store.ActualIsFresh).Should(BeFalse())
	})

	Describe("bumping freshness", func() {
		It("should support bumping freshness by saving a bool and the passed in timestamp", func() {
			err := store.BumpDesiredFreshness(time.Unix(17, 0))
			Ω(store.DesiredIsFresh).Should(BeTrue())
			Ω(store.DesiredFreshnessTimestamp.Equal(time.Unix(17, 0))).Should(BeTrue())
			Ω(err).ShouldNot(HaveOccured())

			err = store.BumpActualFreshness(time.Unix(12, 0))
			Ω(store.ActualIsFresh).Should(BeTrue())
			Ω(store.ActualFreshnessTimestamp.Equal(time.Unix(12, 0))).Should(BeTrue())
			Ω(err).ShouldNot(HaveOccured())

			store.Reset()
			Ω(store.DesiredIsFresh).Should(BeFalse())
			Ω(store.DesiredFreshnessTimestamp).Should(BeZero())
			Ω(store.ActualIsFresh).Should(BeFalse())
			Ω(store.ActualFreshnessTimestamp).Should(BeZero())
		})

		It("should support returning errors", func() {
			errIn := errors.New("foo")
			store.BumpDesiredFreshnessError = errIn
			err := store.BumpDesiredFreshness(time.Unix(17, 0))
			Ω(err).Should(Equal(errIn))

			store.BumpActualFreshnessError = errIn
			err = store.BumpActualFreshness(time.Unix(17, 0))
			Ω(err).Should(Equal(errIn))

			store.Reset()

			err = store.BumpDesiredFreshness(time.Unix(12, 0))
			Ω(err).ShouldNot(HaveOccured())

			err = store.BumpActualFreshness(time.Unix(12, 0))
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
})
