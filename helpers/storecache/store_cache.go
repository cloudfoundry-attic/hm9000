package storecache

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
)

type StoreCache struct {
	store store.Store

	DesiredStates []models.DesiredAppState
	ActualStates  []models.InstanceHeartbeat

	SetOfApps         map[string]bool
	RunningByApp      map[string][]models.InstanceHeartbeat
	DesiredByApp      map[string]models.DesiredAppState
	RunningByInstance map[string]models.InstanceHeartbeat
}

func New(store store.Store) (storecache *StoreCache) {
	return &StoreCache{
		store:             store,
		DesiredStates:     make([]models.DesiredAppState, 0),
		ActualStates:      make([]models.InstanceHeartbeat, 0),
		SetOfApps:         make(map[string]bool, 0),
		RunningByApp:      make(map[string][]models.InstanceHeartbeat, 0),
		DesiredByApp:      make(map[string]models.DesiredAppState, 0),
		RunningByInstance: make(map[string]models.InstanceHeartbeat, 0),
	}
}

func (storecache *StoreCache) Load() (err error) {
	storecache.DesiredStates, err = storecache.store.GetDesiredState()
	if err != nil {
		return err
	}

	storecache.ActualStates, err = storecache.store.GetActualState()
	if err != nil {
		return err
	}

	storecache.SetOfApps = make(map[string]bool, 0)
	storecache.RunningByApp = make(map[string][]models.InstanceHeartbeat, 0)
	storecache.DesiredByApp = make(map[string]models.DesiredAppState, 0)
	storecache.RunningByInstance = make(map[string]models.InstanceHeartbeat, 0)

	for _, desired := range storecache.DesiredStates {
		appKey := storecache.Key(desired.AppGuid, desired.AppVersion)
		storecache.DesiredByApp[appKey] = desired
		storecache.SetOfApps[appKey] = true
	}

	for _, actual := range storecache.ActualStates {
		appKey := storecache.Key(actual.AppGuid, actual.AppVersion)

		storecache.RunningByInstance[actual.InstanceGuid] = actual

		value, ok := storecache.RunningByApp[appKey]
		if ok {
			storecache.RunningByApp[appKey] = append(value, actual)
		} else {
			storecache.RunningByApp[appKey] = []models.InstanceHeartbeat{actual}
		}
		storecache.SetOfApps[appKey] = true
	}

	return nil
}

func (storecache *StoreCache) Key(appGuid string, appVersion string) string {
	return appGuid + "-" + appVersion
}
