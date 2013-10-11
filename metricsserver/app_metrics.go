package metricsserver

import (
	"github.com/cloudfoundry/hm9000/models"
)

type AppMetrics struct {
	desired            models.DesiredAppState
	instanceHeartbeats []models.InstanceHeartbeat

	reportingByIndex map[int]bool

	HasAllInstancesReporting bool
	HasMissingInstances      bool
	IsRunningButUndesired    bool
	NumberOfRunningInstances int
	NumberOfCrashedInstances int
	NumberOfCrashedIndices   int
	NumberOfMissingIndices   int
}

func NewAppMetrics(desired models.DesiredAppState, instanceHeartbeats []models.InstanceHeartbeat) *AppMetrics {
	a := &AppMetrics{desired: desired, instanceHeartbeats: instanceHeartbeats}

	a.reportingByIndex = map[int]bool{}
	for _, heartbeat := range instanceHeartbeats {
		a.reportingByIndex[heartbeat.InstanceIndex] = true
	}

	a.HasAllInstancesReporting, a.HasMissingInstances = a.computeHasAllInstancesReporting()
	a.IsRunningButUndesired = a.computeIsRunningButUndesired()
	a.NumberOfRunningInstances = a.computeNumberOfRunningInstances()
	a.NumberOfCrashedInstances = a.computeNumberOfCrashedInstances()
	a.NumberOfCrashedIndices = a.computeNumberOfCrashedIndices()
	a.NumberOfMissingIndices = a.computeNumberOfMissingIndices()

	return a
}

func (a *AppMetrics) isDesired() bool {
	return a.desired.AppGuid != ""
}

func (a *AppMetrics) computeHasAllInstancesReporting() (bool, bool) {
	if !a.isDesired() {
		return false, false
	}

	for index := 0; index < a.desired.NumberOfInstances; index++ {
		if !a.reportingByIndex[index] {
			return false, true
		}
	}

	return true, false
}

func (a *AppMetrics) computeIsRunningButUndesired() bool {
	if a.isDesired() {
		return false
	}

	for _, heartbeat := range a.instanceHeartbeats {
		if heartbeat.State != models.InstanceStateCrashed {
			return true
		}
	}

	return false
}

func (a *AppMetrics) computeNumberOfRunningInstances() int {
	counter := 0
	for _, heartbeat := range a.instanceHeartbeats {
		if heartbeat.State != models.InstanceStateCrashed {
			counter++
		}
	}

	return counter
}

func (a *AppMetrics) computeNumberOfCrashedInstances() int {
	counter := 0
	for _, heartbeat := range a.instanceHeartbeats {
		if heartbeat.State == models.InstanceStateCrashed {
			counter++
		}
	}

	return counter
}

func (a *AppMetrics) computeNumberOfCrashedIndices() int {
	counter := 0
	crashedAndNotRunning := map[int]bool{}
	for _, heartbeat := range a.instanceHeartbeats {
		if heartbeat.State == models.InstanceStateCrashed {
			crashedAndNotRunning[heartbeat.InstanceIndex] = true
		}
	}

	for _, heartbeat := range a.instanceHeartbeats {
		if heartbeat.State != models.InstanceStateCrashed {
			crashedAndNotRunning[heartbeat.InstanceIndex] = false
		}
	}

	for _, isACrashedIndex := range crashedAndNotRunning {
		if isACrashedIndex {
			counter++
		}
	}

	return counter
}

func (a *AppMetrics) computeNumberOfMissingIndices() int {
	if !a.isDesired() {
		return 0
	}

	counter := 0

	for index := 0; index < a.desired.NumberOfInstances; index++ {
		if !a.reportingByIndex[index] {
			counter++
		}
	}

	return counter
}
