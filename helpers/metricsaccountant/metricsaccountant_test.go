package metricsaccountant_test

import (
	"errors"
	"github.com/cloudfoundry/hm9000/config"
	. "github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/models"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics Accountant", func() {
	var store storepackage.Store
	var accountant MetricsAccountant
	var fakeStoreAdapter *fakestoreadapter.FakeStoreAdapter

	conf, _ := config.DefaultConfig()

	BeforeEach(func() {
		fakeStoreAdapter = fakestoreadapter.New()
		store = storepackage.NewStore(conf, fakeStoreAdapter, fakelogger.NewFakeLogger())
		accountant = New(store)
	})

	Describe("Getting Metrics", func() {
		Context("when the store is empty", func() {
			It("should return a map of 0s", func() {
				metrics, err := accountant.GetMetrics()
				Ω(err).ShouldNot(HaveOccured())
				Ω(metrics).Should(Equal(map[string]int{
					"StartCrashed":           0,
					"StartMissing":           0,
					"StartEvacuating":        0,
					"StopExtra":              0,
					"StopDuplicate":          0,
					"StopEvacuationComplete": 0,
				}))
			})
		})

		Context("when the store errors for some other reason", func() {
			BeforeEach(func() {
				fakeStoreAdapter.GetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("metrics", errors.New("oops"))
			})

			It("should return an error and an empty map", func() {
				metrics, err := accountant.GetMetrics()
				Ω(err).Should(Equal(errors.New("oops")))
				Ω(metrics).Should(BeEmpty())
			})
		})
	})

	Describe("IncrementMetrics", func() {
		var starts []models.PendingStartMessage
		var stops []models.PendingStopMessage
		BeforeEach(func() {
			starts = []models.PendingStartMessage{
				models.PendingStartMessage{StartReason: models.PendingStartMessageReasonCrashed},
				models.PendingStartMessage{StartReason: models.PendingStartMessageReasonMissing},
				models.PendingStartMessage{StartReason: models.PendingStartMessageReasonMissing},
				models.PendingStartMessage{StartReason: models.PendingStartMessageReasonEvacuating},
				models.PendingStartMessage{StartReason: models.PendingStartMessageReasonEvacuating},
				models.PendingStartMessage{StartReason: models.PendingStartMessageReasonEvacuating},
			}

			stops = []models.PendingStopMessage{
				models.PendingStopMessage{StopReason: models.PendingStopMessageReasonExtra},
				models.PendingStopMessage{StopReason: models.PendingStopMessageReasonDuplicate},
				models.PendingStopMessage{StopReason: models.PendingStopMessageReasonDuplicate},
				models.PendingStopMessage{StopReason: models.PendingStopMessageReasonEvacuationComplete},
				models.PendingStopMessage{StopReason: models.PendingStopMessageReasonEvacuationComplete},
				models.PendingStopMessage{StopReason: models.PendingStopMessageReasonEvacuationComplete},
			}
		})

		Context("when the store is empty", func() {
			BeforeEach(func() {
				err := accountant.IncrementMetrics(starts, stops)
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should increment the metrics and return them when GettingMetrics", func() {
				metrics, err := accountant.GetMetrics()
				Ω(err).ShouldNot(HaveOccured())
				Ω(metrics).Should(Equal(map[string]int{
					"StartCrashed":           1,
					"StartMissing":           2,
					"StartEvacuating":        3,
					"StopExtra":              1,
					"StopDuplicate":          2,
					"StopEvacuationComplete": 3,
				}))
			})
		})

		Context("when the metric already exists", func() {
			BeforeEach(func() {
				err := accountant.IncrementMetrics(starts, stops)
				Ω(err).ShouldNot(HaveOccured())
				err = accountant.IncrementMetrics(starts, stops)
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should increment the metrics and return them when GettingMetrics", func() {
				metrics, err := accountant.GetMetrics()
				Ω(err).ShouldNot(HaveOccured())
				Ω(metrics).Should(Equal(map[string]int{
					"StartCrashed":           2,
					"StartMissing":           4,
					"StartEvacuating":        6,
					"StopExtra":              2,
					"StopDuplicate":          4,
					"StopEvacuationComplete": 6,
				}))
			})
		})

		Context("when the store times out while getting metrics", func() {
			BeforeEach(func() {
				fakeStoreAdapter.GetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("metrics", errors.New("oops"))
			})

			It("should return an error", func() {
				err := accountant.IncrementMetrics(starts, stops)
				Ω(err).Should(Equal(errors.New("oops")))
			})
		})

		Context("when the store times out while saving metrics", func() {
			BeforeEach(func() {
				fakeStoreAdapter.SetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("metrics", errors.New("oops"))
			})

			It("should return an error", func() {
				err := accountant.IncrementMetrics(starts, stops)
				Ω(err).Should(Equal(errors.New("oops")))
			})
		})
	})
})
