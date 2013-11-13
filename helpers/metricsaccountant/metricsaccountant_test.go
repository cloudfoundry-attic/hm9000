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
	"time"
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
					"StartCrashed":                            0,
					"StartMissing":                            0,
					"StartEvacuating":                         0,
					"StopExtra":                               0,
					"StopDuplicate":                           0,
					"StopEvacuationComplete":                  0,
					"DesiredStateSyncTimeInMilliseconds":      0,
					"ActualStateListenerStoreUsagePercentage": 0,
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

	Describe("TrackDesiredStateSyncTime", func() {
		It("should record the passed in time duration appropriately", func() {
			err := accountant.TrackDesiredStateSyncTime(1138 * time.Millisecond)
			Ω(err).ShouldNot(HaveOccured())
			metrics, err := accountant.GetMetrics()
			Ω(err).ShouldNot(HaveOccured())
			Ω(metrics["DesiredStateSyncTimeInMilliseconds"]).Should(Equal(1138))
		})
	})

	Describe("TrackActualStateListenerStoreUsageFraction", func() {
		It("should record the passed in time duration appropriately", func() {
			err := accountant.TrackActualStateListenerStoreUsageFraction(0.723)
			Ω(err).ShouldNot(HaveOccured())
			metrics, err := accountant.GetMetrics()
			Ω(err).ShouldNot(HaveOccured())
			Ω(metrics["ActualStateListenerStoreUsagePercentage"]).Should(Equal(72))
		})
	})

	Describe("IncrementSentMessageMetrics", func() {
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
				err := accountant.IncrementSentMessageMetrics(starts, stops)
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should increment the metrics and return them when GettingMetrics", func() {
				metrics, err := accountant.GetMetrics()
				Ω(err).ShouldNot(HaveOccured())
				Ω(metrics["StartCrashed"]).Should(Equal(1))
				Ω(metrics["StartMissing"]).Should(Equal(2))
				Ω(metrics["StartEvacuating"]).Should(Equal(3))
				Ω(metrics["StopExtra"]).Should(Equal(1))
				Ω(metrics["StopDuplicate"]).Should(Equal(2))
				Ω(metrics["StopEvacuationComplete"]).Should(Equal(3))
			})
		})

		Context("when the metric already exists", func() {
			BeforeEach(func() {
				err := accountant.IncrementSentMessageMetrics(starts, stops)
				Ω(err).ShouldNot(HaveOccured())
				err = accountant.IncrementSentMessageMetrics(starts, stops)
				Ω(err).ShouldNot(HaveOccured())
			})

			It("should increment the metrics and return them when GettingMetrics", func() {
				metrics, err := accountant.GetMetrics()
				Ω(err).ShouldNot(HaveOccured())
				Ω(metrics["StartCrashed"]).Should(Equal(2))
				Ω(metrics["StartMissing"]).Should(Equal(4))
				Ω(metrics["StartEvacuating"]).Should(Equal(6))
				Ω(metrics["StopExtra"]).Should(Equal(2))
				Ω(metrics["StopDuplicate"]).Should(Equal(4))
				Ω(metrics["StopEvacuationComplete"]).Should(Equal(6))
			})
		})

		Context("when the store times out while getting metrics", func() {
			BeforeEach(func() {
				fakeStoreAdapter.GetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("metrics", errors.New("oops"))
			})

			It("should return an error", func() {
				err := accountant.IncrementSentMessageMetrics(starts, stops)
				Ω(err).Should(Equal(errors.New("oops")))
			})
		})

		Context("when the store times out while saving metrics", func() {
			BeforeEach(func() {
				fakeStoreAdapter.SetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("metrics", errors.New("oops"))
			})

			It("should return an error", func() {
				err := accountant.IncrementSentMessageMetrics(starts, stops)
				Ω(err).Should(Equal(errors.New("oops")))
			})
		})
	})
})
