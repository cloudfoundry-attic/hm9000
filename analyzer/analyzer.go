package analyzer

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/outbox"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
)

type Analyzer struct {
	store        store.Store
	outbox       outbox.Outbox
	timeProvider timeprovider.TimeProvider
	conf         config.Config

	desiredStates []models.DesiredAppState
	actualStates  []models.InstanceHeartbeat

	setOfApps    map[string]bool
	runningByApp map[string]SortableActualState
	desiredByApp map[string]models.DesiredAppState
}

func New(store store.Store, outbox outbox.Outbox, timeProvider timeprovider.TimeProvider, conf config.Config) *Analyzer {
	return &Analyzer{
		store:        store,
		outbox:       outbox,
		timeProvider: timeProvider,
		conf:         conf,
	}
}

func (analyzer *Analyzer) Analyze() error {
	err := analyzer.fetchStateAndGenerateLookupTables()
	if err != nil {
		return err
	}

	allStartMessages := []models.QueueStartMessage{}
	allStopMessages := []models.QueueStopMessage{}

	for appVersionKey := range analyzer.setOfApps {
		desired := analyzer.desiredByApp[appVersionKey]
		runningInstances, hasRunning := analyzer.runningByApp[appVersionKey]
		if !hasRunning {
			runningInstances = SortableActualState{}
		}
		startMessages, stopMessages := analyzer.analyzeApp(desired, runningInstances)
		allStartMessages = append(allStartMessages, startMessages...)
		allStopMessages = append(allStopMessages, stopMessages...)
	}

	analyzer.outbox.Enqueue(allStartMessages, allStopMessages)
	return nil
}

func (analyzer *Analyzer) fetchStateAndGenerateLookupTables() (err error) {
	desiredStates, err := analyzer.store.GetDesiredState()
	if err != nil {
		return
	}
	analyzer.actualStates, err = analyzer.store.GetActualState()
	if err != nil {
		return
	}

	analyzer.desiredStates = make([]models.DesiredAppState, 0)
	for _, desired := range desiredStates {
		if desired.State == models.AppStateStarted {
			analyzer.desiredStates = append(analyzer.desiredStates, desired)
		}
	}

	analyzer.setOfApps = make(map[string]bool, 0)
	analyzer.desiredByApp = make(map[string]models.DesiredAppState, 0)
	analyzer.runningByApp = make(map[string]SortableActualState, 0)

	for _, desired := range analyzer.desiredStates {
		key := desired.AppGuid + "-" + desired.AppVersion
		analyzer.desiredByApp[key] = desired
		analyzer.setOfApps[key] = true
	}

	for _, actual := range analyzer.actualStates {
		key := actual.AppGuid + "-" + actual.AppVersion
		value, ok := analyzer.runningByApp[key]
		if ok {
			analyzer.runningByApp[key] = append(value, actual)
		} else {
			analyzer.runningByApp[key] = SortableActualState{actual}
		}
		analyzer.setOfApps[key] = true
	}

	return
}
