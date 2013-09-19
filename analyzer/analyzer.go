package analyzer

// very much WIP

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
)

type Analyzer struct {
	storeClient   store.Store
	desiredStates []models.DesiredAppState
	actualStates  []models.InstanceHeartbeat
	runningByApp  map[string]int
	desiredByApp  map[string]bool
}

func New(storeClient store.Store) *Analyzer {
	return &Analyzer{
		storeClient: storeClient,
	}
}

func (analyzer *Analyzer) Analyze() ([]models.QueueStartMessage, []models.QueueStopMessage, error) {
	err := analyzer.populateActualState()
	if err != nil {
		return []models.QueueStartMessage{}, []models.QueueStopMessage{}, err
	}

	err = analyzer.populateDesiredState()
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

func (analyzer *Analyzer) populateDesiredState() error {
	nodes, err := analyzer.fetchNodesUnderDir("/desired")
	if err != nil {
		return err
	}

	analyzer.desiredByApp = make(map[string]bool, 0)

	analyzer.desiredStates = make([]models.DesiredAppState, len(nodes))
	for i, node := range nodes {
		analyzer.desiredStates[i], err = models.NewDesiredAppStateFromJSON(node.Value)
		if err != nil {
			return err
		}

		key := analyzer.desiredStates[i].AppGuid + "-" + analyzer.desiredStates[i].AppVersion
		analyzer.desiredByApp[key] = true
	}

	return nil
}

func (analyzer *Analyzer) populateActualState() error {
	nodes, err := analyzer.fetchNodesUnderDir("/actual")
	if err != nil {
		return err
	}

	analyzer.runningByApp = make(map[string]int, 0)

	analyzer.actualStates = make([]models.InstanceHeartbeat, len(nodes))
	for i, node := range nodes {
		analyzer.actualStates[i], err = models.NewInstanceHeartbeatFromJSON(node.Value)
		if err != nil {
			return err
		}
		key := analyzer.actualStates[i].AppGuid + "-" + analyzer.actualStates[i].AppVersion
		value, ok := analyzer.runningByApp[key]
		if ok {
			analyzer.runningByApp[key] = value + 1
		} else {
			analyzer.runningByApp[key] = 1
		}
	}

	return nil
}

func (analyzer *Analyzer) fetchNodesUnderDir(dir string) ([]store.StoreNode, error) {
	nodes, err := analyzer.storeClient.List(dir)
	if err != nil {
		if store.IsKeyNotFoundError(err) {
			return []store.StoreNode{}, nil
		}
		return []store.StoreNode{}, err
	}
	return nodes, nil
}

func (analyzer *Analyzer) indicesToStart(desiredNumber int) []int {
	arr := make([]int, desiredNumber)
	for i := 0; i < desiredNumber; i++ {
		arr[i] = i
	}
	return arr
}
