package analyzer

import (
	"github.com/cloudfoundry/hm9000/models"
	"strconv"
)

func (analyzer *Analyzer) analyzeApp(desired models.DesiredAppState, runningInstances []models.InstanceHeartbeat) (startMessages []models.QueueStartMessage, stopMessages []models.QueueStopMessage) {
	hasDesired := (desired.AppGuid != "")
	numDesired := 0
	if hasDesired {
		numDesired = desired.NumberOfInstances
	}

	runningByIndex := map[int][]models.InstanceHeartbeat{}
	for _, runningInstance := range runningInstances {
		index := runningInstance.InstanceIndex
		runningByIndex[index] = append(runningByIndex[index], runningInstance)
	}

	//start missing instances
	for index := 0; index < desired.NumberOfInstances; index++ {
		if len(runningByIndex[index]) == 0 {
			analyzer.logger.Info("Identified missing instance", map[string]string{
				"AppGuid":                desired.AppGuid,
				"AppVersion":             desired.AppVersion,
				"InstanceIndex":          strconv.Itoa(index),
				"Desired # of Instances": strconv.Itoa(desired.NumberOfInstances),
			})
			startMessages = append(startMessages, models.NewQueueStartMessage(analyzer.timeProvider.Time(), analyzer.conf.GracePeriod, 0, desired.AppGuid, desired.AppVersion, index))
		}
	}

	if len(startMessages) > 0 {
		return
	}

	//stop extra instances at indices >= numDesired
	for _, runningInstance := range runningInstances {
		if runningInstance.InstanceIndex >= numDesired {
			analyzer.logger.Info("Identified extra running instance", map[string]string{
				"AppGuid":                desired.AppGuid,
				"AppVersion":             desired.AppVersion,
				"InstanceGuid":           runningInstance.InstanceGuid,
				"InstanceIndex":          strconv.Itoa(runningInstance.InstanceIndex),
				"Desired # of Instances": strconv.Itoa(desired.NumberOfInstances),
			})

			stopMessages = append(stopMessages, models.NewQueueStopMessage(analyzer.timeProvider.Time(), 0, analyzer.conf.GracePeriod, runningInstance.InstanceGuid))
		}
	}

	//stop duplicate instances at indices < numDesired
	//this works by scheduling stops for *all* duplicate instances at increasing delays
	//the sender will process the stops one at a time and only send stops that don't put
	//the system in an invalid state
	for index := 0; index < desired.NumberOfInstances; index++ {
		if len(runningByIndex[index]) > 1 {
			duplicateStops := analyzer.stopMessagesForDuplicateInstances(runningByIndex[index])
			stopMessages = append(stopMessages, duplicateStops...)
		}
	}

	return
}

func (analyzer *Analyzer) stopMessagesForDuplicateInstances(runningInstances []models.InstanceHeartbeat) (stopMessages []models.QueueStopMessage) {
	for i, instance := range runningInstances {
		analyzer.logger.Info("Identified duplicate running instance", map[string]string{
			"AppGuid":       instance.AppGuid,
			"AppVersion":    instance.AppVersion,
			"InstanceGuid":  instance.InstanceGuid,
			"InstanceIndex": strconv.Itoa(instance.InstanceIndex),
		})

		stopMessages = append(stopMessages, models.NewQueueStopMessage(analyzer.timeProvider.Time(), (i+1)*analyzer.conf.GracePeriod, analyzer.conf.GracePeriod, instance.InstanceGuid))
	}

	return
}
