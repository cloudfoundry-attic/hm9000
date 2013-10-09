package analyzer

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"math"
	"strconv"
)

type Analyzer struct {
	store      store.Store
	storecache *storecache.StoreCache

	logger       logger.Logger
	timeProvider timeprovider.TimeProvider
	conf         config.Config
}

func New(store store.Store, timeProvider timeprovider.TimeProvider, logger logger.Logger, conf config.Config) *Analyzer {
	return &Analyzer{
		store:        store,
		timeProvider: timeProvider,
		logger:       logger,
		conf:         conf,
		storecache:   storecache.New(store),
	}
}

func (analyzer *Analyzer) Analyze() error {
	err := analyzer.storecache.Load(analyzer.timeProvider.Time())
	if err != nil {
		analyzer.logger.Error("Failed to load desired and actual states", err)
		return err
	}

	allStartMessages := []models.PendingStartMessage{}
	allStopMessages := []models.PendingStopMessage{}
	allCrashCounts := []models.CrashCount{}

	for appVersionKey := range analyzer.storecache.SetOfApps {
		desired := analyzer.storecache.DesiredByApp[appVersionKey]
		heartbeatingInstances := analyzer.storecache.HeartbeatingInstancesByApp[appVersionKey]

		startMessages, stopMessages, crashCounts := analyzer.analyzeApp(desired, heartbeatingInstances)
		allStartMessages = append(allStartMessages, startMessages...)
		allStopMessages = append(allStopMessages, stopMessages...)
		allCrashCounts = append(allCrashCounts, crashCounts...)
	}

	err = analyzer.store.SaveCrashCounts(allCrashCounts)

	if err != nil {
		analyzer.logger.Error("Analyzer failed to save crash counts", err)
		return err
	}

	err = analyzer.store.SavePendingStartMessages(allStartMessages)

	if err != nil {
		analyzer.logger.Error("Analyzer failed to enqueue start messages", err)
		return err
	}

	dedupedMessages := []models.PendingStopMessage{}

	for _, message := range allStopMessages {
		_, found := analyzer.storecache.PendingStopMessages[message.StoreKey()]
		if !found {
			dedupedMessages = append(dedupedMessages, message)
			analyzer.logger.Info("Enqueuing Stop Message", message.LogDescription())
		} else {
			analyzer.logger.Info("Skipping Already Enqueued Stop Message", message.LogDescription())
		}
	}
	err = analyzer.store.SavePendingStopMessages(dedupedMessages)
	if err != nil {
		analyzer.logger.Error("Analyzer failed to enqueue stop messages", err)
		return err
	}

	return nil
}

func (analyzer *Analyzer) analyzeApp(desired models.DesiredAppState, heartbeatingInstances []models.InstanceHeartbeat) (startMessages []models.PendingStartMessage, stopMessages []models.PendingStopMessage, crashCounts []models.CrashCount) {
	runningInstances := []models.InstanceHeartbeat{}
	runningByIndex := map[int][]models.InstanceHeartbeat{}
	numberOfCrashesByIndex := map[int]int{}
	for _, heartbeatingInstance := range heartbeatingInstances {
		index := heartbeatingInstance.InstanceIndex
		if heartbeatingInstance.State == models.InstanceStateCrashed {
			numberOfCrashesByIndex[index] += 1
		} else {
			runningByIndex[index] = append(runningByIndex[index], heartbeatingInstance)
			runningInstances = append(runningInstances, heartbeatingInstance)
		}
	}

	//start missing instances
	priority := analyzer.computePriority(desired.NumberOfInstances, runningByIndex)

	for index := 0; index < desired.NumberOfInstances; index++ {
		if len(runningByIndex[index]) == 0 {
			delay := analyzer.conf.GracePeriod()
			keepAlive := 0
			var crashCount models.CrashCount
			if numberOfCrashesByIndex[index] != 0 {
				previousCrashCount := analyzer.storecache.CrashCount(desired.AppGuid, desired.AppVersion, index)
				delay = analyzer.computeDelayForCrashCount(previousCrashCount)
				keepAlive = analyzer.conf.GracePeriod()

				crashCount = models.CrashCount{
					AppGuid:       desired.AppGuid,
					AppVersion:    desired.AppVersion,
					InstanceIndex: index,
					CrashCount:    previousCrashCount.CrashCount + 1,
				}
			}

			message := models.NewPendingStartMessage(analyzer.timeProvider.Time(), delay, keepAlive, desired.AppGuid, desired.AppVersion, index, priority)
			_, present := analyzer.storecache.PendingStartMessages[message.StoreKey()]

			analyzer.logger.Info("Identified missing instance", message.LogDescription(), map[string]string{
				"Desired # of Instances": strconv.Itoa(desired.NumberOfInstances),
			})

			if !present {
				analyzer.logger.Info("Enqueuing Start Message", message.LogDescription())
				startMessages = append(startMessages, message)
				if crashCount.AppGuid != "" {
					crashCounts = append(crashCounts, crashCount)
				}
			} else {
				analyzer.logger.Info("Skipping Already Enqueued Start Message", message.LogDescription())
			}
		}
	}

	if len(startMessages) > 0 {
		return
	}

	//stop extra instances at indices >= numDesired
	for _, runningInstance := range runningInstances {
		if runningInstance.InstanceIndex >= desired.NumberOfInstances {
			message := models.NewPendingStopMessage(analyzer.timeProvider.Time(), 0, analyzer.conf.GracePeriod(), runningInstance.InstanceGuid)
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

func (analyzer *Analyzer) stopMessagesForDuplicateInstances(runningInstances []models.InstanceHeartbeat) (stopMessages []models.PendingStopMessage) {
	for i, instance := range runningInstances {
		message := models.NewPendingStopMessage(analyzer.timeProvider.Time(), (i+1)*analyzer.conf.GracePeriod(), analyzer.conf.GracePeriod(), instance.InstanceGuid)
		stopMessages = append(stopMessages, message)
		analyzer.logger.Info("Identified duplicate running instance", message.LogDescription(), map[string]string{
			"AppGuid":       instance.AppGuid,
			"AppVersion":    instance.AppVersion,
			"InstanceIndex": strconv.Itoa(instance.InstanceIndex),
		})
	}

	return
}

func (analyzer *Analyzer) computePriority(numDesired int, runningByIndex map[int][]models.InstanceHeartbeat) float64 {
	totalRunningIndices := 0
	for index := 0; index < numDesired; index++ {
		if len(runningByIndex[index]) > 0 {
			totalRunningIndices += 1
		}
	}

	return float64(numDesired-totalRunningIndices) / float64(numDesired)
}

func (analyzer *Analyzer) computeDelayForCrashCount(crashCount models.CrashCount) (delay int) {
	if crashCount.CrashCount < 3 {
		return 0
	}
	if crashCount.CrashCount >= 9 {
		return 960
	}

	return 30 * int(math.Pow(2.0, float64(crashCount.CrashCount-3)))
}
