package analyzer

import (
	"errors"
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
	runningByApp map[string][]models.InstanceHeartbeat
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
	err := analyzer.verifyFreshness()
	if err != nil {
		return err
	}

	err = analyzer.fetchStateAndGenerateLookupTables()
	if err != nil {
		return err
	}

	allStartMessages := []models.QueueStartMessage{}
	allStopMessages := []models.QueueStopMessage{}

	for appVersionKey := range analyzer.setOfApps {
		desired := analyzer.desiredByApp[appVersionKey]
		runningInstances, hasRunning := analyzer.runningByApp[appVersionKey]
		if !hasRunning {
			runningInstances = []models.InstanceHeartbeat{}
		}
		startMessages, stopMessages := analyzer.analyzeApp(desired, runningInstances)
		allStartMessages = append(allStartMessages, startMessages...)
		allStopMessages = append(allStopMessages, stopMessages...)
	}

	analyzer.outbox.Enqueue(allStartMessages, allStopMessages)
	return nil
}

func (analyzer *Analyzer) verifyFreshness() error {
	fresh, err := analyzer.store.IsDesiredStateFresh()
	if err != nil {
		return err
	}
	if !fresh {
		return errors.New("Desired state is not fresh")
	}

	fresh, err = analyzer.store.IsActualStateFresh(analyzer.timeProvider.Time())
	if err != nil {
		return err
	}
	if !fresh {
		return errors.New("Actual state is not fresh")
	}

	return nil
}

func (analyzer *Analyzer) fetchStateAndGenerateLookupTables() (err error) {
	analyzer.desiredStates, err = analyzer.store.GetDesiredState()
	if err != nil {
		return
	}

	analyzer.actualStates, err = analyzer.store.GetActualState()
	if err != nil {
		return
	}

	analyzer.setOfApps = make(map[string]bool, 0)
	analyzer.desiredByApp = make(map[string]models.DesiredAppState, 0)
	analyzer.runningByApp = make(map[string][]models.InstanceHeartbeat, 0)

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
			analyzer.runningByApp[key] = []models.InstanceHeartbeat{actual}
		}
		analyzer.setOfApps[key] = true
	}

	return
}
