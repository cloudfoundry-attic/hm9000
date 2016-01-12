package metricsaccountant_test

import (
	"errors"
	"time"

	"github.com/cloudfoundry/hm9000/config"
	. "github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/models"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
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
				Expect(err).ToNot(HaveOccurred())
				Expect(metrics).To(Equal(map[string]float64{
					"StartCrashed":                            0,
					"StartMissing":                            0,
					"StartEvacuating":                         0,
					"StopExtra":                               0,
					"StopDuplicate":                           0,
					"StopEvacuationComplete":                  0,
					"DesiredStateSyncTimeInMilliseconds":      0,
					"ActualStateListenerStoreUsagePercentage": 0,
					"ReceivedHeartbeats":                      0,
					"SavedHeartbeats":                         0,
				}))
			})
		})

		Context("when the store errors for some other reason", func() {
			BeforeEach(func() {
				fakeStoreAdapter.GetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("metrics", errors.New("oops"))
			})

			It("should return an error and an empty map", func() {
				metrics, err := accountant.GetMetrics()
				Expect(err).To(Equal(errors.New("oops")))
				Expect(metrics).To(BeEmpty())
			})
		})
	})

	Describe("TrackReceivedHeartbeats", func() {
		It("should record the number of received heartbeats appropriately", func() {
			err := accountant.TrackReceivedHeartbeats(127)
			Expect(err).ToNot(HaveOccurred())
			metrics, err := accountant.GetMetrics()
			Expect(err).ToNot(HaveOccurred())
			Expect(metrics["ReceivedHeartbeats"]).To(BeNumerically("==", 127))
		})
	})

	Describe("TrackSavedHeartbeats", func() {
		It("should record the number of received heartbeats appropriately", func() {
			err := accountant.TrackSavedHeartbeats(91)
			Expect(err).ToNot(HaveOccurred())
			metrics, err := accountant.GetMetrics()
			Expect(err).ToNot(HaveOccurred())
			Expect(metrics["SavedHeartbeats"]).To(BeNumerically("==", 91))
		})
	})

	Describe("TrackDesiredStateSyncTime", func() {
		It("should record the passed in time duration appropriately", func() {
			err := accountant.TrackDesiredStateSyncTime(1138 * time.Millisecond)
			Expect(err).ToNot(HaveOccurred())
			metrics, err := accountant.GetMetrics()
			Expect(err).ToNot(HaveOccurred())
			Expect(metrics["DesiredStateSyncTimeInMilliseconds"]).To(BeNumerically("==", 1138))
		})
	})

	Describe("TrackActualStateListenerStoreUsageFraction", func() {
		It("should record the passed in time duration appropriately", func() {
			err := accountant.TrackActualStateListenerStoreUsageFraction(0.723)
			Expect(err).ToNot(HaveOccurred())
			metrics, err := accountant.GetMetrics()
			Expect(err).ToNot(HaveOccurred())
			Expect(metrics["ActualStateListenerStoreUsagePercentage"]).To(BeNumerically("==", 72.3))
		})
	})

	Describe("TrackDesiredStateSyncTime", func() {
		It("should record the passed in time duration appropriately", func() {
			err := accountant.TrackDesiredStateSyncTime(1138 * time.Millisecond)
			Expect(err).ToNot(HaveOccurred())
			metrics, err := accountant.GetMetrics()
			Expect(err).ToNot(HaveOccurred())
			Expect(metrics["DesiredStateSyncTimeInMilliseconds"]).To(BeNumerically("==", 1138))
		})
	})

	Describe("TrackActualStateListenerStoreUsageFraction", func() {
		It("should record the passed in time duration appropriately", func() {
			err := accountant.TrackActualStateListenerStoreUsageFraction(0.723)
			Expect(err).ToNot(HaveOccurred())
			metrics, err := accountant.GetMetrics()
			Expect(err).ToNot(HaveOccurred())
			Expect(metrics["ActualStateListenerStoreUsagePercentage"]).To(BeNumerically("==", 72.3))
		})
	})

	Describe("IncrementSentMessageMetrics", func() {
		var starts []models.PendingStartMessage
		var stops []models.PendingStopMessage
		BeforeEach(func() {
			starts = []models.PendingStartMessage{
				{StartReason: models.PendingStartMessageReasonCrashed},
				{StartReason: models.PendingStartMessageReasonMissing},
				{StartReason: models.PendingStartMessageReasonMissing},
				{StartReason: models.PendingStartMessageReasonEvacuating},
				{StartReason: models.PendingStartMessageReasonEvacuating},
				{StartReason: models.PendingStartMessageReasonEvacuating},
			}

			stops = []models.PendingStopMessage{
				{StopReason: models.PendingStopMessageReasonExtra},
				{StopReason: models.PendingStopMessageReasonDuplicate},
				{StopReason: models.PendingStopMessageReasonDuplicate},
				{StopReason: models.PendingStopMessageReasonEvacuationComplete},
				{StopReason: models.PendingStopMessageReasonEvacuationComplete},
				{StopReason: models.PendingStopMessageReasonEvacuationComplete},
			}
		})

		Context("when the store is empty", func() {
			BeforeEach(func() {
				err := accountant.IncrementSentMessageMetrics(starts, stops)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should increment the metrics and return them when GettingMetrics", func() {
				metrics, err := accountant.GetMetrics()
				Expect(err).ToNot(HaveOccurred())
				Expect(metrics["StartCrashed"]).To(BeNumerically("==", 1))
				Expect(metrics["StartMissing"]).To(BeNumerically("==", 2))
				Expect(metrics["StartEvacuating"]).To(BeNumerically("==", 3))
				Expect(metrics["StopExtra"]).To(BeNumerically("==", 1))
				Expect(metrics["StopDuplicate"]).To(BeNumerically("==", 2))
				Expect(metrics["StopEvacuationComplete"]).To(BeNumerically("==", 3))
			})
		})

		Context("when the metric already exists", func() {
			BeforeEach(func() {
				err := accountant.IncrementSentMessageMetrics(starts, stops)
				Expect(err).ToNot(HaveOccurred())
				err = accountant.IncrementSentMessageMetrics(starts, stops)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should increment the metrics and return them when GettingMetrics", func() {
				metrics, err := accountant.GetMetrics()
				Expect(err).ToNot(HaveOccurred())
				Expect(metrics["StartCrashed"]).To(BeNumerically("==", 2))
				Expect(metrics["StartMissing"]).To(BeNumerically("==", 4))
				Expect(metrics["StartEvacuating"]).To(BeNumerically("==", 6))
				Expect(metrics["StopExtra"]).To(BeNumerically("==", 2))
				Expect(metrics["StopDuplicate"]).To(BeNumerically("==", 4))
				Expect(metrics["StopEvacuationComplete"]).To(BeNumerically("==", 6))
			})
		})

		Context("when the store times out while getting metrics", func() {
			BeforeEach(func() {
				fakeStoreAdapter.GetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("metrics", errors.New("oops"))
			})

			It("should return an error", func() {
				err := accountant.IncrementSentMessageMetrics(starts, stops)
				Expect(err).To(Equal(errors.New("oops")))
			})
		})

		Context("when the store times out while saving metrics", func() {
			BeforeEach(func() {
				fakeStoreAdapter.SetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("metrics", errors.New("oops"))
			})

			It("should return an error", func() {
				err := accountant.IncrementSentMessageMetrics(starts, stops)
				Expect(err).To(Equal(errors.New("oops")))
			})
		})
	})
})
