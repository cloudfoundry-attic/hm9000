package fakestore

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"time"
)

type FakeStore struct {
	ActualIsFresh  bool
	DesiredIsFresh bool

	ActualFreshnessTimestamp  time.Time
	DesiredFreshnessTimestamp time.Time

	BumpDesiredFreshnessError error
	BumpActualFreshnessError  error
	SaveDesiredStateError     error
	GetDesiredStateError      error
	SaveActualStateError      error
	GetActualStateError       error

	desiredState map[string]models.DesiredAppState
	actualState  map[string]models.InstanceHeartbeat
}

func NewFakeStore() *FakeStore {
	store := &FakeStore{}
	store.Reset()
	return store
}

func (store *FakeStore) Reset() {
	store.desiredState = make(map[string]models.DesiredAppState, 0)
	store.actualState = make(map[string]models.InstanceHeartbeat, 0)

	store.ActualIsFresh = false
	store.DesiredIsFresh = false
	store.ActualFreshnessTimestamp = time.Time{}
	store.DesiredFreshnessTimestamp = time.Time{}
	store.BumpActualFreshnessError = nil
	store.BumpDesiredFreshnessError = nil
	store.SaveDesiredStateError = nil
	store.GetDesiredStateError = nil
	store.SaveActualStateError = nil
	store.GetActualStateError = nil
}

func (store *FakeStore) BumpDesiredFreshness(timestamp time.Time) error {
	store.DesiredIsFresh = true
	store.DesiredFreshnessTimestamp = timestamp
	return store.BumpDesiredFreshnessError
}

func (store *FakeStore) BumpActualFreshness(timestamp time.Time) error {
	store.ActualIsFresh = true
	store.ActualFreshnessTimestamp = timestamp
	return store.BumpActualFreshnessError
}

func (store *FakeStore) SaveDesiredState(desiredStates []models.DesiredAppState) error {
	for _, state := range desiredStates {
		store.desiredState[state.StoreKey()] = state
	}
	return store.SaveDesiredStateError
}

func (store *FakeStore) GetDesiredState() ([]models.DesiredAppState, error) {
	if store.GetDesiredStateError != nil {
		return []models.DesiredAppState{}, store.GetDesiredStateError
	}

	desireds := make([]models.DesiredAppState, len(store.desiredState))

	i := 0
	for _, desired := range store.desiredState {
		desireds[i] = desired
		i++
	}

	return desireds, nil
}

func (store *FakeStore) DeleteDesiredState(desiredStates []models.DesiredAppState) error {
	for _, state := range desiredStates {
		_, present := store.desiredState[state.StoreKey()]
		if !present {
			return storeadapter.ErrorKeyNotFound
		}
		delete(store.desiredState, state.StoreKey())
	}
	return nil
}

func (store *FakeStore) SaveActualState(actualStates []models.InstanceHeartbeat) error {
	for _, state := range actualStates {
		store.actualState[state.StoreKey()] = state
	}
	return store.SaveActualStateError
}

func (store *FakeStore) GetActualState() ([]models.InstanceHeartbeat, error) {
	if store.GetActualStateError != nil {
		return []models.InstanceHeartbeat{}, store.GetActualStateError
	}

	actuals := make([]models.InstanceHeartbeat, len(store.actualState))

	i := 0
	for _, actual := range store.actualState {
		actuals[i] = actual
		i++
	}

	return actuals, nil
}

func (store *FakeStore) DeleteActualState(actualStates []models.InstanceHeartbeat) error {
	for _, state := range actualStates {
		_, present := store.actualState[state.StoreKey()]
		if !present {
			return storeadapter.ErrorKeyNotFound
		}
		delete(store.actualState, state.StoreKey())
	}
	return nil
}
