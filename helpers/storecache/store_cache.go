package storecache

import (
	"errors"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"time"
)

type StoreCache struct {
	store store.Store

	DesiredStates        map[string]models.DesiredAppState
	ActualStates         map[string]models.InstanceHeartbeat
	CrashCounts          map[string]models.CrashCount
	PendingStartMessages map[string]models.PendingStartMessage
	PendingStopMessages  map[string]models.PendingStopMessage

	Apps map[string]*models.App

	ActualIsNotFreshError  error
	DesiredIsNotFreshError error
}

func New(store store.Store) (storecache *StoreCache) {
	return &StoreCache{
		store: store,
		ActualIsNotFreshError:  errors.New("Actual state is not fresh"),
		DesiredIsNotFreshError: errors.New("Desired state is not fresh"),
	}
}

func (storecache *StoreCache) Load(time time.Time) (err error) {
	err = storecache.verifyFreshness(time)
	if err != nil {
		return err
	}

	storecache.DesiredStates, err = storecache.store.GetDesiredState()
	if err != nil {
		return err
	}

	storecache.ActualStates, err = storecache.store.GetActualState()
	if err != nil {
		return err
	}

	storecache.CrashCounts, err = storecache.store.GetCrashCounts()
	if err != nil {
		return err
	}

	storecache.PendingStartMessages, err = storecache.store.GetPendingStartMessages()
	if err != nil {
		return err
	}

	storecache.PendingStopMessages, err = storecache.store.GetPendingStopMessages()
	if err != nil {
		return err
	}

	heartbeatingInstancesByApp := make(map[string][]models.InstanceHeartbeat, 0)
	crashCounts := make(map[string]map[int]models.CrashCount)

	for _, actual := range storecache.ActualStates {
		appKey := storecache.Key(actual.AppGuid, actual.AppVersion)
		heartbeatingInstancesByApp[appKey] = append(heartbeatingInstancesByApp[appKey], actual)
	}

	for _, crashCount := range storecache.CrashCounts {
		key := storecache.Key(crashCount.AppGuid, crashCount.AppVersion)
		_, present := crashCounts[key]
		if !present {
			crashCounts[key] = make(map[int]models.CrashCount)
		}
		crashCounts[key][crashCount.InstanceIndex] = crashCount
	}

	storecache.Apps = make(map[string]*models.App, 0)

	for key, desired := range storecache.DesiredStates {
		storecache.Apps[key] = models.NewApp(desired.AppGuid, desired.AppVersion, desired, heartbeatingInstancesByApp[key], crashCounts[key])
	}

	for key, instances := range heartbeatingInstancesByApp {
		_, present := storecache.Apps[key]
		if !present {
			storecache.Apps[key] = models.NewApp(instances[0].AppGuid, instances[0].AppVersion, models.DesiredAppState{}, instances, crashCounts[key])
		}
	}

	return nil
}

func (storecache *StoreCache) Key(appGuid string, appVersion string) string {
	return appGuid + "-" + appVersion
}

func (storecache *StoreCache) verifyFreshness(time time.Time) error {
	fresh, err := storecache.store.IsDesiredStateFresh()
	if err != nil {
		return err
	}
	if !fresh {
		return storecache.DesiredIsNotFreshError
	}

	fresh, err = storecache.store.IsActualStateFresh(time)
	if err != nil {
		return err
	}
	if !fresh {
		return storecache.ActualIsNotFreshError
	}

	return nil
}
