package analyzer

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/models"
	"strconv"
	"time"
)

type appAnalyzer struct {
	heartbeatingInstances []models.InstanceHeartbeat
	desired               models.DesiredAppState
	conf                  config.Config
	storecache            *storecache.StoreCache
	currentTime           time.Time
	logger                logger.Logger

	runningInstances []models.InstanceHeartbeat
	runningByIndex   map[int][]models.InstanceHeartbeat
	hasCrashAtIndex  map[int]bool
}

func newAppAnalyzer(desired models.DesiredAppState, heartbeatingInstances []models.InstanceHeartbeat, currentTime time.Time, storecache *storecache.StoreCache, logger logger.Logger, conf config.Config) *appAnalyzer {
	return &appAnalyzer{
		heartbeatingInstances: heartbeatingInstances,
		desired:               desired,
		conf:                  conf,
		storecache:            storecache,
		currentTime:           currentTime,
		logger:                logger,
	}
}

func (a *appAnalyzer) analyzeApp() (startMessages []models.PendingStartMessage, stopMessages []models.PendingStopMessage, crashCounts []models.CrashCount) {
	a.partitionInstancesIntoRunningAndCrashed()

	startMessages, crashCounts = a.generatePendingStartsForMissingInstances()

	if len(startMessages) > 0 {
		return
	}

	stopMessages = append(stopMessages, a.generatePendingStopsForExtraInstances()...)
	stopMessages = append(stopMessages, a.generatePendingStopsForDuplicateInstances()...)

	stopMessages = a.dedupeStopMessages(stopMessages)

	return
}

func (a *appAnalyzer) partitionInstancesIntoRunningAndCrashed() {
	a.runningInstances = []models.InstanceHeartbeat{}
	a.runningByIndex = map[int][]models.InstanceHeartbeat{}
	a.hasCrashAtIndex = map[int]bool{}

	for _, heartbeatingInstance := range a.heartbeatingInstances {
		index := heartbeatingInstance.InstanceIndex
		if heartbeatingInstance.State == models.InstanceStateCrashed {
			a.hasCrashAtIndex[index] = true
		} else {
			a.runningByIndex[index] = append(a.runningByIndex[index], heartbeatingInstance)
			a.runningInstances = append(a.runningInstances, heartbeatingInstance)
		}
	}
}

func (a *appAnalyzer) generatePendingStartsForMissingInstances() (startMessages []models.PendingStartMessage, crashCounts []models.CrashCount) {
	priority := a.computePendingStartMessagePriority()

	for index := 0; a.isIndexDesired(index); index++ {
		if a.hasNoRunningInstancesAtIndex(index) {
			delay := a.conf.GracePeriod()
			keepAlive := 0
			var crashCount models.CrashCount
			if a.hasCrashAtIndex[index] {
				crashCount = a.storecache.CrashCount(a.desired.AppGuid, a.desired.AppVersion, index)
				delay = a.computeDelayForCrashCount(crashCount)
				keepAlive = a.conf.GracePeriod()
			}

			message := models.NewPendingStartMessage(a.currentTime, delay, keepAlive, a.desired.AppGuid, a.desired.AppVersion, index, priority)

			a.logger.Info("Identified missing instance", message.LogDescription(), map[string]string{
				"Desired # of Instances": strconv.Itoa(a.desired.NumberOfInstances),
			})

			_, present := a.storecache.PendingStartMessages[message.StoreKey()]
			if !present {
				a.logger.Info("Enqueuing Start Message", message.LogDescription())
				startMessages = append(startMessages, message)
				if a.hasCrashAtIndex[index] {
					crashCount.CrashCount += 1
					crashCounts = append(crashCounts, crashCount)
				}
			} else {
				a.logger.Info("Skipping Already Enqueued Start Message", message.LogDescription())
			}
		}
	}

	return
}

func (a *appAnalyzer) generatePendingStopsForExtraInstances() (stopMessages []models.PendingStopMessage) {
	for _, runningInstance := range a.runningInstances {
		if !a.isIndexDesired(runningInstance.InstanceIndex) {
			message := models.NewPendingStopMessage(a.currentTime, 0, a.conf.GracePeriod(), runningInstance.InstanceGuid)

			stopMessages = append(stopMessages, message)

			a.logger.Info("Identified extra running instance", message.LogDescription(), map[string]string{
				"AppGuid":                a.desired.AppGuid,
				"AppVersion":             a.desired.AppVersion,
				"InstanceIndex":          strconv.Itoa(runningInstance.InstanceIndex),
				"Desired # of Instances": strconv.Itoa(a.desired.NumberOfInstances),
			})
		}
	}

	return
}

func (a *appAnalyzer) generatePendingStopsForDuplicateInstances() (stopMessages []models.PendingStopMessage) {
	//stop duplicate instances at indices < numDesired
	//this works by scheduling stops for *all* duplicate instances at increasing delays
	//the sender will process the stops one at a time and only send stops that don't put
	//the system in an invalid state
	for index := 0; a.isIndexDesired(index); index++ {
		if a.hasDuplicateRunningInstancesAtIndex(index) {
			for i, instance := range a.runningByIndex[index] {
				delay := (i + 1) * a.conf.GracePeriod()
				message := models.NewPendingStopMessage(a.currentTime, delay, a.conf.GracePeriod(), instance.InstanceGuid)

				stopMessages = append(stopMessages, message)

				a.logger.Info("Identified duplicate running instance", message.LogDescription(), map[string]string{
					"AppGuid":       instance.AppGuid,
					"AppVersion":    instance.AppVersion,
					"InstanceIndex": strconv.Itoa(instance.InstanceIndex),
				})
			}
		}
	}

	return
}

func (a *appAnalyzer) dedupeStopMessages(stopMessages []models.PendingStopMessage) (dedupedMessages []models.PendingStopMessage) {
	for _, message := range stopMessages {
		_, present := a.storecache.PendingStopMessages[message.StoreKey()]
		if !present {
			a.logger.Info("Enqueuing Stop Message", message.LogDescription())
			dedupedMessages = append(dedupedMessages, message)
		} else {
			a.logger.Info("Skipping Already Enqueued Stop Message", message.LogDescription())
		}
	}

	return
}

func (a *appAnalyzer) isIndexDesired(instanceIndex int) bool {
	return instanceIndex < a.desired.NumberOfInstances
}

func (a *appAnalyzer) hasDuplicateRunningInstancesAtIndex(instanceIndex int) bool {
	return len(a.runningByIndex[instanceIndex]) > 1
}

func (a *appAnalyzer) hasNoRunningInstancesAtIndex(instanceIndex int) bool {
	return len(a.runningByIndex[instanceIndex]) == 0
}

func (a *appAnalyzer) computePendingStartMessagePriority() float64 {
	totalRunningIndices := 0
	for index := 0; a.isIndexDesired(index); index++ {
		if len(a.runningByIndex[index]) > 0 {
			totalRunningIndices += 1
		}
	}

	return float64(a.desired.NumberOfInstances-totalRunningIndices) / float64(a.desired.NumberOfInstances)
}

func (a *appAnalyzer) computeDelayForCrashCount(crashCount models.CrashCount) (delay int) {
	startingBackoffDelay := int(a.conf.StartingBackoffDelay().Seconds())
	maximumBackoffDelay := int(a.conf.MaximumBackoffDelay().Seconds())
	return ComputeCrashDelay(crashCount.CrashCount, a.conf.NumberOfCrashesBeforeBackoffBegins, startingBackoffDelay, maximumBackoffDelay)
}
