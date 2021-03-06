// This file was generated by counterfeiter
package fakemetricsaccountant

import (
	"sync"
	"time"

	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
)

type FakeUsageTracker struct {
	StartTrackingUsageStub        func()
	startTrackingUsageMutex       sync.RWMutex
	startTrackingUsageArgsForCall []struct{}
	MeasureUsageStub              func() (usage float64, measurementDuration time.Duration)
	measureUsageMutex             sync.RWMutex
	measureUsageArgsForCall       []struct{}
	measureUsageReturns           struct {
		result1 float64
		result2 time.Duration
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeUsageTracker) StartTrackingUsage() {
	fake.startTrackingUsageMutex.Lock()
	fake.startTrackingUsageArgsForCall = append(fake.startTrackingUsageArgsForCall, struct{}{})
	fake.recordInvocation("StartTrackingUsage", []interface{}{})
	fake.startTrackingUsageMutex.Unlock()
	if fake.StartTrackingUsageStub != nil {
		fake.StartTrackingUsageStub()
	}
}

func (fake *FakeUsageTracker) StartTrackingUsageCallCount() int {
	fake.startTrackingUsageMutex.RLock()
	defer fake.startTrackingUsageMutex.RUnlock()
	return len(fake.startTrackingUsageArgsForCall)
}

func (fake *FakeUsageTracker) MeasureUsage() (usage float64, measurementDuration time.Duration) {
	fake.measureUsageMutex.Lock()
	fake.measureUsageArgsForCall = append(fake.measureUsageArgsForCall, struct{}{})
	fake.recordInvocation("MeasureUsage", []interface{}{})
	fake.measureUsageMutex.Unlock()
	if fake.MeasureUsageStub != nil {
		return fake.MeasureUsageStub()
	} else {
		return fake.measureUsageReturns.result1, fake.measureUsageReturns.result2
	}
}

func (fake *FakeUsageTracker) MeasureUsageCallCount() int {
	fake.measureUsageMutex.RLock()
	defer fake.measureUsageMutex.RUnlock()
	return len(fake.measureUsageArgsForCall)
}

func (fake *FakeUsageTracker) MeasureUsageReturns(result1 float64, result2 time.Duration) {
	fake.MeasureUsageStub = nil
	fake.measureUsageReturns = struct {
		result1 float64
		result2 time.Duration
	}{result1, result2}
}

func (fake *FakeUsageTracker) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.startTrackingUsageMutex.RLock()
	defer fake.startTrackingUsageMutex.RUnlock()
	fake.measureUsageMutex.RLock()
	defer fake.measureUsageMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeUsageTracker) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ metricsaccountant.UsageTracker = new(FakeUsageTracker)
