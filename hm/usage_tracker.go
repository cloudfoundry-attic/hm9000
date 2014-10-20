package hm

import (
	"sync"
	"time"
)

type usageTracker struct {
	workerCount      int
	lock             sync.Mutex
	startTime        time.Time
	timeSpentWorking time.Duration
}

func newUsageTracker(workerCount int) *usageTracker {
	return &usageTracker{
		workerCount: workerCount,
		startTime:   time.Now(),
	}
}

func (u *usageTracker) Around(work func()) {
	start := time.Now()
	work()
	workTime := time.Since(start)

	u.lock.Lock()
	u.timeSpentWorking += workTime
	u.lock.Unlock()
}

func (u *usageTracker) StartTrackingUsage() {
	u.resetUsageMetrics()
}

func (u *usageTracker) MeasureUsage() (float64, time.Duration) {
	timeSpentWorking, timeSinceStartTime := u.resetUsageMetrics()
	usage := timeSpentWorking / (timeSinceStartTime.Seconds() * float64(u.workerCount))
	return usage, timeSinceStartTime
}

func (u *usageTracker) resetUsageMetrics() (float64, time.Duration) {
	u.lock.Lock()
	t := time.Now()

	lastStart := u.startTime
	u.startTime = t

	timeSpentWorking := u.timeSpentWorking
	u.timeSpentWorking = 0

	u.lock.Unlock()

	return timeSpentWorking.Seconds(), t.Sub(lastStart)
}
