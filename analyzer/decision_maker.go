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
	totalRunning := 0
	for index := 0; index < desired.NumberOfInstances; index++ {
		if len(runningByIndex[index]) > 0 {
			totalRunning += 1
		}
	}
	priority := float64(desired.NumberOfInstances-totalRunning) / float64(desired.NumberOfInstances)

	for index := 0; index < desired.NumberOfInstances; index++ {
		if len(runningByIndex[index]) == 0 {
			message := models.NewQueueStartMessage(analyzer.timeProvider.Time(), analyzer.conf.GracePeriod(), 0, desired.AppGuid, desired.AppVersion, index, priority)
			startMessages = append(startMessages, message)
			analyzer.logger.Info("Identified missing instance", message.LogDescription(), map[string]string{
				"Desired # of Instances": strconv.Itoa(desired.NumberOfInstances),
			})
		}
	}

	if len(startMessages) > 0 {
		return
	}

	//stop extra instances at indices >= numDesired
	for _, runningInstance := range runningInstances {
		if runningInstance.InstanceIndex >= numDesired {
			message := models.NewQueueStopMessage(analyzer.timeProvider.Time(), 0, analyzer.conf.GracePeriod(), runningInstance.InstanceGuid)
			stopMessages = append(stopMessages, message)
			analyzer.logger.Info("Identified extra running instance", message.LogDescription(), map[string]string{
				"AppGuid":                desired.AppGuid,
				"AppVersion":             desired.AppVersion,
				"InstanceIndex":          strconv.Itoa(runningInstance.InstanceIndex),
				"Desired # of Instances": strconv.Itoa(desired.NumberOfInstances),
			})
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
		message := models.NewQueueStopMessage(analyzer.timeProvider.Time(), (i+1)*analyzer.conf.GracePeriod(), analyzer.conf.GracePeriod(), instance.InstanceGuid)
		stopMessages = append(stopMessages, message)
		analyzer.logger.Info("Identified duplicate running instance", message.LogDescription(), map[string]string{
			"AppGuid":       instance.AppGuid,
			"AppVersion":    instance.AppVersion,
			"InstanceIndex": strconv.Itoa(instance.InstanceIndex),
		})
	}

	return
}
