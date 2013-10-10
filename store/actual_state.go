package store

import (
	"github.com/cloudfoundry/hm9000/models"
	"reflect"
)

func (store *RealStore) SaveActualState(actualStates ...models.InstanceHeartbeat) error {
	return store.save(actualStates, "/actual", store.config.HeartbeatTTL())
}

func (store *RealStore) GetActualState() ([]models.InstanceHeartbeat, error) {
	slice, err := store.get("/actual", reflect.TypeOf([]models.InstanceHeartbeat{}), reflect.ValueOf(models.NewInstanceHeartbeatFromJSON))
	return slice.Interface().([]models.InstanceHeartbeat), err
}

func (store *RealStore) DeleteActualState(actualStates ...models.InstanceHeartbeat) error {
	return store.delete(actualStates, "/actual")
}
