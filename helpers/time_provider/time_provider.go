package time_provider

import "time"

type TimeProvider interface {
	Time() time.Time
}

type RealTimeProvider struct{}

func (provider *RealTimeProvider) Time() time.Time {
	return time.Now()
}
