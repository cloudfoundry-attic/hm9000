package store

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
)

func (store *RealStore) SaveActualState(actualStates []models.InstanceHeartbeat) error {
	nodes := make([]storeadapter.StoreNode, len(actualStates))
	for i, actualState := range actualStates {
		nodes[i] = storeadapter.StoreNode{
			Key:   "/actual/" + actualState.StoreKey(),
			Value: actualState.ToJSON(),
			TTL:   store.config.HeartbeatTTL,
		}
	}

	return store.adapter.Set(nodes)
}

func (store *RealStore) GetActualState() ([]models.InstanceHeartbeat, error) {
	nodes, err := store.fetchNodesUnderDir("/actual")
	if err != nil {
		return []models.InstanceHeartbeat{}, err
	}

	instanceHeartbeats := make([]models.InstanceHeartbeat, len(nodes))
	for i, node := range nodes {
		instanceHeartbeats[i], err = models.NewInstanceHeartbeatFromJSON(node.Value)
		if err != nil {
			return []models.InstanceHeartbeat{}, err
		}
	}

	return instanceHeartbeats, nil
}

func (store *RealStore) DeleteActualState(actualStates []models.InstanceHeartbeat) error {
	for _, actualState := range actualStates {
		err := store.adapter.Delete("/actual/" + actualState.StoreKey())
		if err != nil {
			return err
		}
	}
	return nil
}
