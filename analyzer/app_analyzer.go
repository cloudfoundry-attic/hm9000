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

	startMessages []models.PendingStartMessage
	stopMessages  []models.PendingStopMessage
	crashCounts   []models.CrashCount

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
		startMessages:         make([]models.PendingStartMessage, 0),
		stopMessages:          make([]models.PendingStopMessage, 0),
		crashCounts:           make([]models.CrashCount, 0),
	}
}

func (a *appAnalyzer) analyzeApp() ([]models.PendingStartMessage, []models.PendingStopMessage, []models.CrashCount) {
	a.partitionInstancesIntoRunningAndCrashed()

	a.generatePendingStartsForMissingInstances()
	a.generatePendingStartsForCrashedInstances()

	if len(a.startMessages) == 0 {
		a.generatePendingStopsForExtraInstances()
		a.generatePendingStopsForDuplicateInstances()
	}

	return a.startMessages, a.stopMessages, a.crashCounts
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

func (a *appAnalyzer) generatePendingStartsForMissingInstances() {
	priority := a.computePendingStartMessagePriority()

	for index := 0; a.isIndexDesired(index); index++ {
		if a.hasNoRunningInstancesAtIndex(index) && !a.hasCrashAtIndex[index] {
			message := models.NewPendingStartMessage(a.currentTime, a.conf.GracePeriod(), 0, a.desired.AppGuid, a.desired.AppVersion, index, priority)

			a.logger.Info("Identified missing instance", message.LogDescription(), map[string]string{
				"Desired # of Instances": strconv.Itoa(a.desired.NumberOfInstances),
			})

			a.appendStartMessageIfNotDuplicate(message)
		}
	}

	return
}

func (a *appAnalyzer) generatePendingStartsForCrashedInstances() (crashCounts []models.CrashCount) {
	priority := a.computePendingStartMessagePriority()

	for index := 0; a.isIndexDesired(index); index++ {
		if a.hasNoRunningInstancesAtIndex(index) && a.hasCrashAtIndex[index] {
			if index != 0 && a.hasNoRunningInstance() {
				continue
			}

			crashCount := a.storecache.CrashCount(a.desired.AppGuid, a.desired.AppVersion, index)
			delay := a.computeDelayForCrashCount(crashCount)
			message := models.NewPendingStartMessage(a.currentTime, delay, a.conf.GracePeriod(), a.desired.AppGuid, a.desired.AppVersion, index, priority)

			a.logger.Info("Identified crashed instance", message.LogDescription(), map[string]string{
				"Desired # of Instances": strconv.Itoa(a.desired.NumberOfInstances),
			})

			didAppend := a.appendStartMessageIfNotDuplicate(message)

			if didAppend {
				crashCount.CrashCount += 1
				a.crashCounts = append(a.crashCounts, crashCount)
			}
		}
	}

	return
}

func (a *appAnalyzer) generatePendingStopsForExtraInstances() {
	for _, runningInstance := range a.runningInstances {
		if !a.isIndexDesired(runningInstance.InstanceIndex) {
			message := models.NewPendingStopMessage(a.currentTime, 0, a.conf.GracePeriod(), runningInstance.InstanceGuid)

			a.appendStopMessageIfNotDuplicate(message)

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

func (a *appAnalyzer) generatePendingStopsForDuplicateInstances() {
	//stop duplicate instances at indices < numDesired
	//this works by scheduling stops for *all* duplicate instances at increasing delays
	//the sender will process the stops one at a time and only send stops that don't put
	//the system in an invalid state
	for index := 0; a.isIndexDesired(index); index++ {
		if a.hasDuplicateRunningInstancesAtIndex(index) {
			for i, instance := range a.runningByIndex[index] {
				delay := (i + 1) * a.conf.GracePeriod()
				message := models.NewPendingStopMessage(a.currentTime, delay, a.conf.GracePeriod(), instance.InstanceGuid)

				a.appendStopMessageIfNotDuplicate(message)

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

func (a *appAnalyzer) appendStartMessageIfNotDuplicate(message models.PendingStartMessage) (didAppend bool) {
	_, alreadyQueued := a.storecache.PendingStartMessages[message.StoreKey()]
	if !alreadyQueued {
		a.logger.Info("Enqueuing Start Message", message.LogDescription())
		a.startMessages = append(a.startMessages, message)
		return true
	} else {
		a.logger.Info("Skipping Already Enqueued Start Message", message.LogDescription())
		return false
	}
}

func (a *appAnalyzer) appendStopMessageIfNotDuplicate(message models.PendingStopMessage) {
	_, alreadyQueued := a.storecache.PendingStopMessages[message.StoreKey()]
	if !alreadyQueued {
		a.logger.Info("Enqueuing Stop Message", message.LogDescription())
		a.stopMessages = append(a.stopMessages, message)
	} else {
		a.logger.Info("Skipping Already Enqueued Stop Message", message.LogDescription())
	}
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

func (a *appAnalyzer) hasNoRunningInstance() bool {
	return len(a.runningInstances) == 0
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
