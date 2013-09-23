package analyzer

// very much WIP
// needs to handle many actually doing the diff

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
)

type Analyzer struct {
	store         store.Store
	desiredStates []models.DesiredAppState
	actualStates  []models.InstanceHeartbeat
	runningByApp  map[string][]models.InstanceHeartbeat
	desiredByApp  map[string]models.DesiredAppState
}

func New(store store.Store) *Analyzer {
	return &Analyzer{
		store: store,
	}
}

func (analyzer *Analyzer) Analyze() ([]models.QueueStartMessage, []models.QueueStopMessage, error) {
	err := analyzer.fetchStateAndGenerateLookupTables()
	if err != nil {
		return []models.QueueStartMessage{}, []models.QueueStopMessage{}, err
	}

	startMessages := analyzer.startMessagesForMissingInstances()
	stopMessages := analyzer.stopMessagesForExtraInstances()

	return startMessages, stopMessages, nil
}

func (analyzer *Analyzer) fetchStateAndGenerateLookupTables() (err error) {
	desiredStates, err := analyzer.store.GetDesiredState()
	if err != nil {
		return
	}
	analyzer.actualStates, err = analyzer.store.GetActualState()
	if err != nil {
		return err
	}

	analyzer.desiredStates = make([]models.DesiredAppState, 0)
	for _, desired := range desiredStates {
		if desired.State == models.AppStateStarted {
			analyzer.desiredStates = append(analyzer.desiredStates, desired)
		}
	}

	analyzer.desiredByApp = make(map[string]models.DesiredAppState, 0)
	for _, desired := range analyzer.desiredStates {
		key := desired.AppGuid + "-" + desired.AppVersion
		analyzer.desiredByApp[key] = desired
	}

	analyzer.runningByApp = make(map[string][]models.InstanceHeartbeat, 0)
	for _, actual := range analyzer.actualStates {
		key := actual.AppGuid + "-" + actual.AppVersion
		value, ok := analyzer.runningByApp[key]
		if ok {
			analyzer.runningByApp[key] = append(value, actual)
		} else {
			analyzer.runningByApp[key] = []models.InstanceHeartbeat{actual}
		}
	}

	return
}

func (analyzer *Analyzer) startMessagesForMissingInstances() []models.QueueStartMessage {
	startMessages := make([]models.QueueStartMessage, 0)
	for _, desired := range analyzer.desiredStates {
		runningInstances, ok := analyzer.runningByApp[desired.AppGuid+"-"+desired.AppVersion]
		if !ok {
			runningInstances = []models.InstanceHeartbeat{}
		}

		if len(runningInstances) < desired.NumberOfInstances {
			startMessages = append(startMessages, models.QueueStartMessage{
				AppGuid:        desired.AppGuid,
				AppVersion:     desired.AppVersion,
				IndicesToStart: analyzer.indicesToStart(desired.NumberOfInstances, runningInstances),
			})
			continue
		}
	}

	return startMessages
}

func (analyzer *Analyzer) stopMessagesForExtraInstances() []models.QueueStopMessage {
	stopMessages := make([]models.QueueStopMessage, 0)
	for _, runningInstance := range analyzer.actualStates {
		desired, ok := analyzer.desiredByApp[runningInstance.AppGuid+"-"+runningInstance.AppVersion]
		if ok && desired.NumberOfInstances > runningInstance.InstanceIndex {
			continue
		}

		stopMessages = append(stopMessages, models.QueueStopMessage{
			InstanceGuid: runningInstance.InstanceGuid,
		})
	}

	return stopMessages
}

func (analyzer *Analyzer) indicesToStart(desiredNumber int, runningInstances []models.InstanceHeartbeat) []int {
	runningIndices := map[int]bool{}
	for _, runningInstance := range runningInstances {
		runningIndices[runningInstance.InstanceIndex] = true
	}

	arr := []int{}
	for i := 0; i < desiredNumber; i++ {
		_, ok := runningIndices[i]
		if !ok {
			arr = append(arr, i)
		}
	}
	return arr
}
