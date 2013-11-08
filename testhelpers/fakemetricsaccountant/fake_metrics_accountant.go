package fakemetricsaccountant

import (
	"github.com/cloudfoundry/hm9000/models"
)

type FakeMetricsAccountant struct {
	IncrementMetricsError error
	IncrementedStarts     []models.PendingStartMessage
	IncrementedStops      []models.PendingStopMessage

	GetMetricsError   error
	GetMetricsMetrics map[string]int
}

func New() *FakeMetricsAccountant {
	return &FakeMetricsAccountant{
		IncrementedStarts: []models.PendingStartMessage{},
		IncrementedStops:  []models.PendingStopMessage{},

		GetMetricsMetrics: map[string]int{},
	}
}

func (m *FakeMetricsAccountant) IncrementMetrics(starts []models.PendingStartMessage, stops []models.PendingStopMessage) error {
	m.IncrementedStarts = starts
	m.IncrementedStops = stops

	return m.IncrementMetricsError
}

func (m *FakeMetricsAccountant) GetMetrics() (map[string]int, error) {
	return m.GetMetricsMetrics, m.GetMetricsError
}
