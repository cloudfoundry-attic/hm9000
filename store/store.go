package store

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"time"
)

type Storeable interface {
	StoreKey() string
	ToJSON() []byte
}

type Store interface {
	BumpDesiredFreshness(timestamp time.Time) error
	BumpActualFreshness(timestamp time.Time) error

	SaveDesiredState(desiredStates []models.DesiredAppState) error
	GetDesiredState() ([]models.DesiredAppState, error)
	DeleteDesiredState(desiredStates []models.DesiredAppState) error

	SaveActualState(actualStates []models.InstanceHeartbeat) error
	GetActualState() ([]models.InstanceHeartbeat, error)
	DeleteActualState(actualStates []models.InstanceHeartbeat) error

	SaveQueueStartMessages(startMessages []models.QueueStartMessage) error
	GetQueueStartMessages() ([]models.QueueStartMessage, error)
	DeleteQueueStartMessages(startMessages []models.QueueStartMessage) error

	SaveQueueStopMessages(stopMessages []models.QueueStopMessage) error
	GetQueueStopMessages() ([]models.QueueStopMessage, error)
	DeleteQueueStopMessages(stopMessages []models.QueueStopMessage) error
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
