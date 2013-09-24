package store

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
)

func (store *RealStore) SaveDesiredState(desiredStates []models.DesiredAppState) error {
	nodes := make([]storeadapter.StoreNode, len(desiredStates))
	for i, desiredState := range desiredStates {
		nodes[i] = storeadapter.StoreNode{
			Key:   "/desired/" + desiredState.StoreKey(),
			Value: desiredState.ToJSON(),
			TTL:   store.config.DesiredStateTTL,
		}
	}

	return store.adapter.Set(nodes)
}

func (store *RealStore) GetDesiredState() ([]models.DesiredAppState, error) {
	nodes, err := store.fetchNodesUnderDir("/desired")
	if err != nil {
		return []models.DesiredAppState{}, err
	}

	desiredStates := make([]models.DesiredAppState, len(nodes))
	for i, node := range nodes {
		desiredStates[i], err = models.NewDesiredAppStateFromJSON(node.Value)
		if err != nil {
			return []models.DesiredAppState{}, err
		}
	}

	return desiredStates, nil
}

func (store *RealStore) DeleteDesiredState(desiredStates []models.DesiredAppState) error {
	for _, desiredState := range desiredStates {
		err := store.adapter.Delete("/desired/" + desiredState.StoreKey())
		if err != nil {
			return err
		}
	}
	return nil
}
