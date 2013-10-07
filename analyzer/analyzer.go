package analyzer

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/outbox"
	"github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"strconv"
)

type Analyzer struct {
	store      store.Store
	storecache *storecache.StoreCache

	logger       logger.Logger
	outbox       outbox.Outbox
	timeProvider timeprovider.TimeProvider
	conf         config.Config
}

func New(store store.Store, outbox outbox.Outbox, timeProvider timeprovider.TimeProvider, logger logger.Logger, conf config.Config) *Analyzer {
	return &Analyzer{
		store:        store,
		outbox:       outbox,
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

	for appVersionKey := range analyzer.storecache.SetOfApps {
		desired := analyzer.storecache.DesiredByApp[appVersionKey]
		heartbeatingInstances := analyzer.storecache.HeartbeatingInstancesByApp[appVersionKey]

		startMessages, stopMessages := analyzer.analyzeApp(desired, heartbeatingInstances)
		allStartMessages = append(allStartMessages, startMessages...)
		allStopMessages = append(allStopMessages, stopMessages...)
	}

	err = analyzer.outbox.Enqueue(allStartMessages, allStopMessages)
	if err != nil {
		analyzer.logger.Error("Analyzer failed to enqueue messages", err)
		return err
	}
	return nil
}

func (analyzer *Analyzer) analyzeApp(desired models.DesiredAppState, heartbeatingInstances []models.InstanceHeartbeat) (startMessages []models.PendingStartMessage, stopMessages []models.PendingStopMessage) {
	hasDesired := (desired.AppGuid != "")
	numDesired := 0
	if hasDesired {
		numDesired = desired.NumberOfInstances
	}

	runningInstances := []models.InstanceHeartbeat{}
	runningByIndex := map[int][]models.InstanceHeartbeat{}
	for _, heartbeatingInstance := range heartbeatingInstances {
		if heartbeatingInstance.State != models.InstanceStateCrashed {
			index := heartbeatingInstance.InstanceIndex
			runningByIndex[index] = append(runningByIndex[index], heartbeatingInstance)
			runningInstances = append(runningInstances, heartbeatingInstance)
		}
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
			message := models.NewPendingStartMessage(analyzer.timeProvider.Time(), analyzer.conf.GracePeriod(), 0, desired.AppGuid, desired.AppVersion, index, priority)
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
