package fakestore

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"time"
)

type FakeStore struct {
	ActualFreshnessTimestamp           time.Time
	DesiredFreshnessTimestamp          time.Time
	ActualFreshnessComparisonTimestamp time.Time

	BumpDesiredFreshnessError error
	BumpActualFreshnessError  error
	IsDesiredStateFreshError  error
	IsActualStateFreshError   error

	SaveDesiredStateError    error
	GetDesiredStateError     error
	SaveActualStateError     error
	GetActualStateError      error
	SaveStartMessagesError   error
	GetStartMessagesError    error
	DeleteStartMessagesError error
	SaveStopMessagesError    error
	GetStopMessagesError     error
	DeleteStopMessagesError  error
	SaveCrashCountsError     error
	GetCrashCountsError      error
	DeleteCrashCountsError   error

	desiredState  map[string]models.DesiredAppState
	actualState   map[string]models.InstanceHeartbeat
	startMessages map[string]models.PendingStartMessage
	stopMessages  map[string]models.PendingStopMessage
	crashCounts   map[string]models.CrashCount
}

func NewFakeStore() *FakeStore {
	store := &FakeStore{}
	store.Reset()
	return store
}

func (store *FakeStore) Reset() {
	store.desiredState = make(map[string]models.DesiredAppState, 0)
	store.actualState = make(map[string]models.InstanceHeartbeat, 0)
	store.startMessages = make(map[string]models.PendingStartMessage, 0)
	store.stopMessages = make(map[string]models.PendingStopMessage, 0)
	store.crashCounts = make(map[string]models.CrashCount, 0)

	store.ActualFreshnessTimestamp = time.Time{}
	store.DesiredFreshnessTimestamp = time.Time{}
	store.ActualFreshnessComparisonTimestamp = time.Time{}

	store.BumpDesiredFreshnessError = nil
	store.BumpActualFreshnessError = nil
	store.IsDesiredStateFreshError = nil
	store.IsActualStateFreshError = nil

	store.SaveDesiredStateError = nil
	store.GetDesiredStateError = nil
	store.SaveActualStateError = nil
	store.GetActualStateError = nil
	store.SaveStartMessagesError = nil
	store.GetStartMessagesError = nil
	store.DeleteStartMessagesError = nil
	store.SaveStopMessagesError = nil
	store.GetStopMessagesError = nil
	store.DeleteStopMessagesError = nil
	store.SaveCrashCountsError = nil
	store.GetCrashCountsError = nil
	store.DeleteCrashCountsError = nil
}

func (store *FakeStore) BumpDesiredFreshness(timestamp time.Time) error {
	store.DesiredFreshnessTimestamp = timestamp
	return store.BumpDesiredFreshnessError
}

func (store *FakeStore) BumpActualFreshness(timestamp time.Time) error {
	store.ActualFreshnessTimestamp = timestamp
	return store.BumpActualFreshnessError
}

func (store *FakeStore) IsDesiredStateFresh() (bool, error) {
	return store.DesiredFreshnessTimestamp != time.Time{}, store.IsDesiredStateFreshError
}

func (store *FakeStore) IsActualStateFresh(timestamp time.Time) (bool, error) {
	store.ActualFreshnessComparisonTimestamp = timestamp
	return store.ActualFreshnessTimestamp != time.Time{}, store.IsActualStateFreshError
}

func (store *FakeStore) SaveDesiredState(desiredStates ...models.DesiredAppState) error {
	for _, state := range desiredStates {
		store.desiredState[state.StoreKey()] = state
	}
	return store.SaveDesiredStateError
}

func (store *FakeStore) GetDesiredState() (map[string]models.DesiredAppState, error) {
	if store.GetDesiredStateError != nil {
		return map[string]models.DesiredAppState{}, store.GetDesiredStateError
	}

	return store.desiredState, nil
}

func (store *FakeStore) DeleteDesiredState(desiredStates ...models.DesiredAppState) error {
	for _, state := range desiredStates {
		_, present := store.desiredState[state.StoreKey()]
		if !present {
			return storeadapter.ErrorKeyNotFound
		}
		delete(store.desiredState, state.StoreKey())
	}
	return nil
}

func (store *FakeStore) SaveActualState(actualStates ...models.InstanceHeartbeat) error {
	for _, state := range actualStates {
		store.actualState[state.StoreKey()] = state
	}
	return store.SaveActualStateError
}

func (store *FakeStore) GetActualState() (map[string]models.InstanceHeartbeat, error) {
	if store.GetActualStateError != nil {
		return map[string]models.InstanceHeartbeat{}, store.GetActualStateError
	}

	return store.actualState, nil
}

func (store *FakeStore) DeleteActualState(actualStates ...models.InstanceHeartbeat) error {
	for _, state := range actualStates {
		_, present := store.actualState[state.StoreKey()]
		if !present {
			return storeadapter.ErrorKeyNotFound
		}
		delete(store.actualState, state.StoreKey())
	}
	return nil
}

func (store *FakeStore) SavePendingStartMessages(messages ...models.PendingStartMessage) error {
	for _, message := range messages {
		store.startMessages[message.StoreKey()] = message
	}
	return store.SaveStartMessagesError
}

func (store *FakeStore) GetPendingStartMessages() (map[string]models.PendingStartMessage, error) {
	if store.GetStartMessagesError != nil {
		return map[string]models.PendingStartMessage{}, store.GetStartMessagesError
	}
	return store.startMessages, nil
}

func (store *FakeStore) DeletePendingStartMessages(messages ...models.PendingStartMessage) error {
	for _, message := range messages {
		_, present := store.startMessages[message.StoreKey()]
		if !present {
			return storeadapter.ErrorKeyNotFound
		}
		delete(store.startMessages, message.StoreKey())
	}
	return store.DeleteStartMessagesError
}

func (store *FakeStore) SavePendingStopMessages(messages ...models.PendingStopMessage) error {
	for _, message := range messages {
		store.stopMessages[message.StoreKey()] = message
	}
	return store.SaveStopMessagesError
}

func (store *FakeStore) GetPendingStopMessages() (map[string]models.PendingStopMessage, error) {
	if store.GetStopMessagesError != nil {
		return map[string]models.PendingStopMessage{}, store.GetStopMessagesError
	}
	return store.stopMessages, nil
}

func (store *FakeStore) DeletePendingStopMessages(messages ...models.PendingStopMessage) error {
	for _, message := range messages {
		_, present := store.stopMessages[message.StoreKey()]
		if !present {
			return storeadapter.ErrorKeyNotFound
		}
		delete(store.stopMessages, message.StoreKey())
	}
	return store.DeleteStopMessagesError
}

func (store *FakeStore) SaveCrashCounts(crashCounts ...models.CrashCount) error {
	for _, crashCount := range crashCounts {
		store.crashCounts[crashCount.StoreKey()] = crashCount
	}
	return store.SaveCrashCountsError
}

func (store *FakeStore) GetCrashCounts() (map[string]models.CrashCount, error) {
	if store.GetCrashCountsError != nil {
		return map[string]models.CrashCount{}, store.GetCrashCountsError
	}
	return store.crashCounts, nil
}

func (store *FakeStore) DeleteCrashCounts(crashCounts ...models.CrashCount) error {
	for _, crashCount := range crashCounts {
		_, present := store.crashCounts[crashCount.StoreKey()]
		if !present {
			return storeadapter.ErrorKeyNotFound
		}
		delete(store.crashCounts, crashCount.StoreKey())
	}
	return store.DeleteCrashCountsError
}
