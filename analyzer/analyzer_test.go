package analyzer_test

// very much WIP

import (
	"errors"
	. "github.com/cloudfoundry/hm9000/analyzer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestore"
)

var _ = Describe("Analyzer", func() {
	var (
		analyzer *Analyzer
		store    *fakestore.FakeStore
		app1     app.App
		app2     app.App
	)

	BeforeEach(func() {
		store = fakestore.NewFakeStore()

		app1 = app.NewApp()
		app2 = app.NewApp()

		analyzer = New(store)
	})

	Context("When fetching desired state fails with an error", func() {
		BeforeEach(func() {
			store.GetDesiredStateError = errors.New("oops!")
		})

		It("Should not send any start or stop messages", func() {
			startMessages, stopMessages, err := analyzer.Analyze()
			Ω(err).Should(Equal(errors.New("oops!")))
			Ω(startMessages).Should(BeEmpty())
			Ω(stopMessages).Should(BeEmpty())
		})
	})

	Context("When fetching actual state fails with an error", func() {
		BeforeEach(func() {
			store.GetActualStateError = errors.New("oops!")
		})

		It("Should not send any start or stop messages", func() {
			startMessages, stopMessages, err := analyzer.Analyze()
			Ω(err).Should(Equal(errors.New("oops!")))
			Ω(startMessages).Should(BeEmpty())
			Ω(stopMessages).Should(BeEmpty())
		})
	})

	Context("When there are no desired or running apps", func() {
		It("Should not send any start or stop messages", func() {
			startMessages, stopMessages, err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
			Ω(startMessages).Should(BeEmpty())
			Ω(stopMessages).Should(BeEmpty())
		})
	})

	Context("where thare are desired instances but no running instances", func() {
		BeforeEach(func() {
			desired1 := app1.DesiredState(17)
			desired1.NumberOfInstances = 1
			desired2 := app2.DesiredState(34)
			desired2.NumberOfInstances = 3
			store.SaveDesiredState([]models.DesiredAppState{
				desired1,
				desired2,
			})
		})

		It("Should return an array of start messages for the missing instances", func() {
			startMessages, stopMessages, err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
			Ω(stopMessages).Should(BeEmpty())
			Ω(startMessages).Should(HaveLen(2))
			Ω(startMessages).Should(ContainElement(models.QueueStartMessage{
				AppGuid:        app1.AppGuid,
				AppVersion:     app1.AppVersion,
				IndicesToStart: []int{0},
			}))
			Ω(startMessages).Should(ContainElement(models.QueueStartMessage{
				AppGuid:        app2.AppGuid,
				AppVersion:     app2.AppVersion,
				IndicesToStart: []int{0, 1, 2},
			}))
		})
	})

	Context("When there are running instances but no desired instances", func() {
		BeforeEach(func() {
			store.SaveActualState([]models.InstanceHeartbeat{
				app1.GetInstance(0).Heartbeat(12),
				app2.GetInstance(0).Heartbeat(17),
				app2.GetInstance(2).Heartbeat(1138),
			})
		})

		It("Should return an array of stop messages for the extra instances", func() {
			startMessages, stopMessages, err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
			Ω(startMessages).Should(BeEmpty())
			Ω(stopMessages).Should(HaveLen(3))
			Ω(stopMessages).Should(ContainElement(models.QueueStopMessage{
				InstanceGuid: app1.GetInstance(0).InstanceGuid,
			}))
			Ω(stopMessages).Should(ContainElement(models.QueueStopMessage{
				InstanceGuid: app2.GetInstance(0).InstanceGuid,
			}))
			Ω(stopMessages).Should(ContainElement(models.QueueStopMessage{
				InstanceGuid: app2.GetInstance(2).InstanceGuid,
			}))
		})
	})

	Context("When there is one desired instance which is running", func() {
		BeforeEach(func() {
			desired := app1.DesiredState(10)
			desired.NumberOfInstances = 1
			store.SaveDesiredState([]models.DesiredAppState{
				desired,
			})
			store.SaveActualState([]models.InstanceHeartbeat{
				app1.GetInstance(0).Heartbeat(12),
			})
		})

		It("Should not send any start or stop messages", func() {
			startMessages, stopMessages, err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
			Ω(startMessages).Should(BeEmpty())
			Ω(stopMessages).Should(BeEmpty())
		})
	})
})
