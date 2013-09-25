package store

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"reflect"
	"time"
)

type Storeable interface {
	StoreKey() string
	ToJSON() []byte
}

type Store interface {
	BumpDesiredFreshness(timestamp time.Time) error
	BumpActualFreshness(timestamp time.Time) error

	IsDesiredStateFresh() (bool, error)
	IsActualStateFresh(time.Time) (bool, error)

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

// buckle up, here be dragons...

func (store *RealStore) save(stuff interface{}, root string, ttl uint64) error {
	arrValue := reflect.ValueOf(stuff)

	nodes := make([]storeadapter.StoreNode, arrValue.Len())
	for i := 0; i < arrValue.Len(); i++ {
		item := arrValue.Index(i).Interface().(Storeable)
		nodes[i] = storeadapter.StoreNode{
			Key:   root + "/" + item.StoreKey(),
			Value: item.ToJSON(),
			TTL:   ttl,
		}
	}

	return store.adapter.Set(nodes)
}

func (store *RealStore) get(root string, sliceType reflect.Type, constructor reflect.Value) (reflect.Value, error) {
	nodes, err := store.fetchNodesUnderDir(root)
	if err != nil {
		return reflect.MakeSlice(sliceType, 0, 0), err
	}

	slice := reflect.MakeSlice(sliceType, 0, 0)
	for _, node := range nodes {
		out := constructor.Call([]reflect.Value{reflect.ValueOf(node.Value)})
		slice = reflect.Append(slice, out[0])
		if !out[1].IsNil() {
			return reflect.MakeSlice(sliceType, 0, 0), out[1].Interface().(error)
		}
	}

	return slice, nil
}

func (store *RealStore) delete(stuff interface{}, root string) error {
	arrValue := reflect.ValueOf(stuff)

	for i := 0; i < arrValue.Len(); i++ {
		item := arrValue.Index(i).Interface().(Storeable)

		err := store.adapter.Delete(root + "/" + item.StoreKey())
		if err != nil {
			return err
		}
	}
	return nil
}
