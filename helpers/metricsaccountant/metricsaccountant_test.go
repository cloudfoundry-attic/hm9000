package metricsaccountant_test

import (
	"time"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
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
	var sender *fake.FakeMetricSender

	conf, _ := config.DefaultConfig()

	BeforeEach(func() {
		fakeStoreAdapter = fakestoreadapter.New()
		store = storepackage.NewStore(conf, fakeStoreAdapter, fakelogger.NewFakeLogger())
		accountant = New()

		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, nil)
	})

	Describe("TrackReceivedHeartbeats", func() {
		It("should record the number of received heartbeats appropriately", func() {
			err := accountant.TrackReceivedHeartbeats(127)
			Expect(err).ToNot(HaveOccurred())
			hbs := sender.GetValue(ReceivedHeartbeats)
			Expect(hbs).To(Equal(fake.Metric{127, "Metric"}))
		})
	})

	Describe("TrackSavedHeartbeats", func() {
		It("should record the number of received heartbeats appropriately", func() {
			err := accountant.TrackSavedHeartbeats(91)
			Expect(err).ToNot(HaveOccurred())
			hbs := sender.GetValue(SavedHeartbeats)
			Expect(hbs).To(Equal(fake.Metric{91, "Metric"}))
		})
	})

	Describe("TrackDesiredStateSyncTime", func() {
		It("should record the passed in time duration appropriately", func() {
			err := accountant.TrackDesiredStateSyncTime(1138 * time.Millisecond)
			Expect(err).ToNot(HaveOccurred())
			dst := sender.GetValue(DesiredStateSyncTime)
			Expect(dst).To(Equal(fake.Metric{1138, "ms"}))
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
				Expect(sender.GetCounter(StartCrashed)).To(Equal(uint64(1)))
				Expect(sender.GetCounter(StartMissing)).To(Equal(uint64(2)))
				Expect(sender.GetCounter(StartEvacuating)).To(Equal(uint64(3)))
				Expect(sender.GetCounter(StopExtra)).To(Equal(uint64(1)))
				Expect(sender.GetCounter(StopDuplicate)).To(Equal(uint64(2)))
				Expect(sender.GetCounter(StopEvacuationComplete)).To(Equal(uint64(3)))
			})

			Context("when the metric already exists", func() {
				BeforeEach(func() {
					err := accountant.IncrementSentMessageMetrics(starts, stops)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should increment the metrics and return them when GettingMetrics", func() {
					Expect(sender.GetCounter(StartCrashed)).To(Equal(uint64(2)))
					Expect(sender.GetCounter(StartMissing)).To(Equal(uint64(4)))
					Expect(sender.GetCounter(StartEvacuating)).To(Equal(uint64(6)))
					Expect(sender.GetCounter(StopExtra)).To(Equal(uint64(2)))
					Expect(sender.GetCounter(StopDuplicate)).To(Equal(uint64(4)))
					Expect(sender.GetCounter(StopEvacuationComplete)).To(Equal(uint64(6)))
				})
			})
		})
	})
})
