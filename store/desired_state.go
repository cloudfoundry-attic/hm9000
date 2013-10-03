package store

import (
	"github.com/cloudfoundry/hm9000/models"
	"reflect"
)

func (store *RealStore) SaveDesiredState(desiredStates []models.DesiredAppState) error {
	return store.save(desiredStates, "/desired", store.config.DesiredStateTTL())
}

func (store *RealStore) GetDesiredState() ([]models.DesiredAppState, error) {
	slice, err := store.get("/desired", reflect.TypeOf([]models.DesiredAppState{}), reflect.ValueOf(models.NewDesiredAppStateFromJSON))
	return slice.Interface().([]models.DesiredAppState), err
}

func (store *RealStore) DeleteDesiredState(desiredStates []models.DesiredAppState) error {
	return store.delete(desiredStates, "/desired")
}
