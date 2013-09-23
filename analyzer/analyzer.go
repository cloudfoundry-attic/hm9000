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
	runningByApp  map[string]int
	desiredByApp  map[string]bool
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

	startMessages := make([]models.QueueStartMessage, 0)
	for _, state := range analyzer.desiredStates {
		key := state.AppGuid + "-" + state.AppVersion
		if analyzer.runningByApp[key] == 0 {
			startMessage := models.QueueStartMessage{
				AppGuid:        state.AppGuid,
				AppVersion:     state.AppVersion,
				IndicesToStart: analyzer.indicesToStart(state.NumberOfInstances),
			}
			startMessages = append(startMessages, startMessage)
		}
	}

	stopMessages := make([]models.QueueStopMessage, 0)
	for _, state := range analyzer.actualStates {
		key := state.AppGuid + "-" + state.AppVersion
		if !analyzer.desiredByApp[key] {
			stopMessage := models.QueueStopMessage{
				InstanceGuid: state.InstanceGuid,
			}
			stopMessages = append(stopMessages, stopMessage)
		}
	}

	return startMessages, stopMessages, nil
}

func (analyzer *Analyzer) fetchStateAndGenerateLookupTables() (err error) {
	analyzer.desiredStates, err = analyzer.store.GetDesiredState()
	if err != nil {
		return
	}
	analyzer.actualStates, err = analyzer.store.GetActualState()
	if err != nil {
		return err
	}

	analyzer.desiredByApp = make(map[string]bool, 0)
	for _, desired := range analyzer.desiredStates {
		key := desired.AppGuid + "-" + desired.AppVersion
		analyzer.desiredByApp[key] = true
	}

	analyzer.runningByApp = make(map[string]int, 0)
	for _, actual := range analyzer.actualStates {
		key := actual.AppGuid + "-" + actual.AppVersion
		value, ok := analyzer.runningByApp[key]
		if ok {
			analyzer.runningByApp[key] = value + 1
		} else {
			analyzer.runningByApp[key] = 1
		}
	}

	return
}

func (analyzer *Analyzer) indicesToStart(desiredNumber int) []int {
	arr := make([]int, desiredNumber)
	for i := 0; i < desiredNumber; i++ {
		arr[i] = i
	}
	return arr
}
