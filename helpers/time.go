package helpers

import "time"

type TimeProvider interface {
	Time() time.Time
}

type RealTimeProvider struct{}

func (provider *RealTimeProvider) Time() time.Time {
	return time.Now()
}

type FakeTimeProvider struct {
	TimeToProvide time.Time
}

func (provider *FakeTimeProvider) Time() time.Time {
	return provider.TimeToProvide
}
