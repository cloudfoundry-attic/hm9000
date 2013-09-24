package analyzer

import (
	"github.com/cloudfoundry/hm9000/models"
)

func analyzeApp(desired models.DesiredAppState, runningInstances SortableActualState) (startMessages []models.QueueStartMessage, stopMessages []models.QueueStopMessage) {
	hasDesired := (desired.AppGuid != "")
	numDesired := 0
	if hasDesired {
		numDesired = desired.NumberOfInstances
	}

	runningIndices := map[int]bool{}
	for _, runningInstance := range runningInstances {
		runningIndices[runningInstance.InstanceIndex] = true
	}

	indicesToStart := []int{}
	for index := 0; index < desired.NumberOfInstances; index++ {
		if !runningIndices[index] {
			indicesToStart = append(indicesToStart, index)
		}
	}

	if len(indicesToStart) > 0 {
		startMessages = append(startMessages, models.QueueStartMessage{
			AppGuid:        desired.AppGuid,
			AppVersion:     desired.AppVersion,
			IndicesToStart: indicesToStart,
		})
	}

	if len(runningInstances) > numDesired {
		runningInstances.SortDescendingInPlace()
		numToStop := len(runningInstances) - numDesired
		for i := 0; i < numToStop; i++ {
			stopMessages = append(stopMessages, models.QueueStopMessage{
				InstanceGuid: runningInstances[i].InstanceGuid,
			})
		}
	}

	return
}
