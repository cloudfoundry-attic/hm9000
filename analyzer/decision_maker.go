package analyzer

import (
	"github.com/cloudfoundry/hm9000/models"
)

func (analyzer *Analyzer) analyzeApp(desired models.DesiredAppState, runningInstances SortableActualState) (startMessages []models.QueueStartMessage, stopMessages []models.QueueStopMessage) {
	hasDesired := (desired.AppGuid != "")
	numDesired := 0
	if hasDesired {
		numDesired = desired.NumberOfInstances
	}

	runningIndices := map[int]bool{}
	for _, runningInstance := range runningInstances {
		runningIndices[runningInstance.InstanceIndex] = true
	}

	for index := 0; index < desired.NumberOfInstances; index++ {
		if !runningIndices[index] {
			startMessages = append(startMessages, models.NewQueueStartMessage(analyzer.timeProvider.Time(), analyzer.conf.GracePeriod, 0, desired.AppGuid, desired.AppVersion, index))
		}
	}

	if len(runningInstances) > numDesired {
		runningInstances.SortDescendingInPlace()
		numToStop := len(runningInstances) - numDesired
		for i := 0; i < numToStop; i++ {
			stopMessages = append(stopMessages, models.NewQueueStopMessage(analyzer.timeProvider.Time(), 0, analyzer.conf.GracePeriod, runningInstances[i].InstanceGuid))
		}
	}

	return
}
