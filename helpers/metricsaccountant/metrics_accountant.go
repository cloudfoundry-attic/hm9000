package metricsaccountant

import (
	"time"

	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/hm9000/models"
)

const (
	ReceivedHeartbeats     = "ReceivedHeartbeats"
	SavedHeartbeats        = "SavedHeartbeats"
	StartCrashed           = "StartCrashed"
	StartMissing           = "StartMissing"
	StartEvacuating        = "StartEvacuating"
	StopExtra              = "StopExtra"
	StopDuplicate          = "StopDuplicate"
	StopEvacuationComplete = "StopEvacuationComplete"
)

//go:generate counterfeiter -o fakemetricsaccountant/fake_usagetracker.go . UsageTracker
type UsageTracker interface {
	StartTrackingUsage()
	MeasureUsage() (usage float64, measurementDuration time.Duration)
}

var startMetrics = map[models.PendingStartMessageReason]string{
	models.PendingStartMessageReasonCrashed:    StartCrashed,
	models.PendingStartMessageReasonMissing:    StartMissing,
	models.PendingStartMessageReasonEvacuating: StartEvacuating,
}

var stopMetrics = map[models.PendingStopMessageReason]string{
	models.PendingStopMessageReasonDuplicate:          StopDuplicate,
	models.PendingStopMessageReasonExtra:              StopExtra,
	models.PendingStopMessageReasonEvacuationComplete: StopEvacuationComplete,
}

//go:generate counterfeiter -o fakemetricsaccountant/fake_metricsaccountant.go . MetricsAccountant
type MetricsAccountant interface {
	TrackReceivedHeartbeats(metric int) error
	TrackSavedHeartbeats(metric int) error
	IncrementSentMessageMetrics(starts []models.PendingStartMessage, stops []models.PendingStopMessage) error
}

type RealMetricsAccountant struct {
}

func New() *RealMetricsAccountant {
	return &RealMetricsAccountant{}
}

func (m *RealMetricsAccountant) TrackReceivedHeartbeats(metric int) error {
	return metrics.SendValue(ReceivedHeartbeats, float64(metric), "Metric")
}

func (m *RealMetricsAccountant) TrackSavedHeartbeats(metric int) error {
	return metrics.SendValue(SavedHeartbeats, float64(metric), "Metric")
}

func (m *RealMetricsAccountant) IncrementSentMessageMetrics(starts []models.PendingStartMessage, stops []models.PendingStopMessage) error {
	counts := make(map[string]uint64)

	for _, start := range starts {
		addOne(counts, startMetrics[start.StartReason])
	}

	for _, stop := range stops {
		addOne(counts, stopMetrics[stop.StopReason])
	}

	for key, value := range counts {
		err := metrics.AddToCounter(key, value)
		if err != nil {
			return err
		}
	}

	return nil
}

func addOne(m map[string]uint64, key string) {
	v := m[key]
	m[key] = v + 1
}
