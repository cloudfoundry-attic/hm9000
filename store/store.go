package store

import (
	"encoding/json"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"time"
)

type Store interface {
	BumpDesiredFreshness(timestamp time.Time) error
	BumpActualFreshness(timestamp time.Time) error
	SaveDesiredState(desiredStates []models.DesiredAppState) error
	GetDesiredState() ([]models.DesiredAppState, error)
	DeleteDesiredState(desiredStates []models.DesiredAppState) error
	SaveActualState(actualStates []models.InstanceHeartbeat) error
	GetActualState() ([]models.InstanceHeartbeat, error)
	DeleteActualState(actualStates []models.InstanceHeartbeat) error
}

type RealStore struct {
	config  config.Config
	adapter storeadapter.StoreAdapter
}

func NewStore(config config.Config, adapter storeadapter.StoreAdapter) *RealStore {
	return &RealStore{
		config:  config,
		adapter: adapter,
	}
}

func (store *RealStore) BumpDesiredFreshness(timestamp time.Time) error {
	return store.bumpFreshness(store.config.DesiredFreshnessKey, store.config.DesiredFreshnessTTL, timestamp)
}

func (store *RealStore) BumpActualFreshness(timestamp time.Time) error {
	return store.bumpFreshness(store.config.ActualFreshnessKey, store.config.ActualFreshnessTTL, timestamp)
}

func (store *RealStore) SaveDesiredState(desiredStates []models.DesiredAppState) error {
	nodes := make([]storeadapter.StoreNode, len(desiredStates))
	for i, desiredState := range desiredStates {
		nodes[i] = storeadapter.StoreNode{
			Key:   "/desired/" + desiredState.StoreKey(),
			Value: desiredState.ToJson(),
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

func (store *RealStore) SaveActualState(actualStates []models.InstanceHeartbeat) error {
	nodes := make([]storeadapter.StoreNode, len(actualStates))
	for i, actualState := range actualStates {
		nodes[i] = storeadapter.StoreNode{
			Key:   "/actual/" + actualState.StoreKey(),
			Value: actualState.ToJson(),
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

func (store *RealStore) fetchNodesUnderDir(dir string) ([]storeadapter.StoreNode, error) {
	nodes, err := store.adapter.List(dir)
	if err != nil {
		if err == storeadapter.ErrorKeyNotFound {
			return []storeadapter.StoreNode{}, nil
		}
		return []storeadapter.StoreNode{}, err
	}
	return nodes, nil
}

func (store *RealStore) bumpFreshness(key string, ttl uint64, timestamp time.Time) error {
	var jsonTimestamp []byte
	oldTimestamp, err := store.adapter.Get(key)

	if err == nil {
		jsonTimestamp = oldTimestamp.Value
	} else {
		jsonTimestamp, _ = json.Marshal(models.FreshnessTimestamp{Timestamp: timestamp.Unix()})
	}

	return store.adapter.Set([]storeadapter.StoreNode{
		storeadapter.StoreNode{
			Key:   key,
			Value: jsonTimestamp,
			TTL:   ttl,
		},
	})
}
