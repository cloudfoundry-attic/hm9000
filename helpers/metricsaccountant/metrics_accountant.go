package metricsaccountant

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"
)

var startMetrics = map[models.PendingStartMessageReason]string{
	models.PendingStartMessageReasonCrashed:    "StartCrashed",
	models.PendingStartMessageReasonMissing:    "StartMissing",
	models.PendingStartMessageReasonEvacuating: "StartEvacuating",
}

var stopMetrics = map[models.PendingStopMessageReason]string{
	models.PendingStopMessageReasonDuplicate:          "StopDuplicate",
	models.PendingStopMessageReasonExtra:              "StopExtra",
	models.PendingStopMessageReasonEvacuationComplete: "StopEvacuationComplete",
}

type MetricsAccountant interface {
	IncrementMetrics(starts []models.PendingStartMessage, stops []models.PendingStopMessage) error
	GetMetrics() (map[string]int, error)
}

type RealMetricsAccountant struct {
	store store.Store
}

func New(store store.Store) *RealMetricsAccountant {
	return &RealMetricsAccountant{
		store: store,
	}
}

func (m *RealMetricsAccountant) IncrementMetrics(starts []models.PendingStartMessage, stops []models.PendingStopMessage) error {
	metrics, err := m.GetMetrics()
	if err != nil {
		return err
	}

	for _, start := range starts {
		metrics[startMetrics[start.StartReason]] += 1
	}

	for _, stop := range stops {
		metrics[stopMetrics[stop.StopReason]] += 1
	}

	for key, value := range metrics {
		err := m.store.SaveMetric(key, value)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *RealMetricsAccountant) GetMetrics() (map[string]int, error) {
	metrics := map[string]int{}
	for _, key := range startMetrics {
		metrics[key] = 0
	}
	for _, key := range stopMetrics {
		metrics[key] = 0
	}

	for key, _ := range metrics {
		value, err := m.store.GetMetric(key)
		if err == storeadapter.ErrorKeyNotFound {
			value = 0
		} else if err != nil {
			return map[string]int{}, err
		}
		metrics[key] = value
	}

	return metrics, nil
}
