package storecache

import (
	"errors"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"time"
)

type StoreCache struct {
	store store.Store

	DesiredStates []models.DesiredAppState
	ActualStates  []models.InstanceHeartbeat
	CrashCounts   []models.CrashCount

	Apps                 map[string]*models.App
	AppsByInstanceGuid   map[string]*models.App
	PendingStartMessages map[string]models.PendingStartMessage
	PendingStopMessages  map[string]models.PendingStopMessage
}

func New(store store.Store) (storecache *StoreCache) {
	return &StoreCache{
		store: store,
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

	pendingStartMessages, err := storecache.store.GetPendingStartMessages()
	if err != nil {
		return err
	}

	pendingStopMessages, err := storecache.store.GetPendingStopMessages()
	if err != nil {
		return err
	}

	heartbeatingInstancesByApp := make(map[string][]models.InstanceHeartbeat, 0)
	desiredByApp := make(map[string]models.DesiredAppState, 0)
	crashCounts := make(map[string]map[int]models.CrashCount)

	for _, desired := range storecache.DesiredStates {
		appKey := storecache.Key(desired.AppGuid, desired.AppVersion)
		desiredByApp[appKey] = desired
	}

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

	for key, desired := range desiredByApp {
		storecache.Apps[key] = models.NewApp(desired.AppGuid, desired.AppVersion, desired, heartbeatingInstancesByApp[key], crashCounts[key])
	}

	for key, instances := range heartbeatingInstancesByApp {
		_, present := storecache.Apps[key]
		if !present {
			storecache.Apps[key] = models.NewApp(instances[0].AppGuid, instances[0].AppVersion, models.DesiredAppState{}, instances, crashCounts[key])
		}
	}

	storecache.AppsByInstanceGuid = make(map[string]*models.App, 0)
	for _, app := range storecache.Apps {
		for _, heartbeat := range app.InstanceHeartbeats {
			storecache.AppsByInstanceGuid[heartbeat.InstanceGuid] = app
		}
	}

	storecache.PendingStartMessages = make(map[string]models.PendingStartMessage, 0)
	storecache.PendingStopMessages = make(map[string]models.PendingStopMessage, 0)

	for _, m := range pendingStartMessages {
		storecache.PendingStartMessages[m.StoreKey()] = m
	}

	for _, m := range pendingStopMessages {
		storecache.PendingStopMessages[m.StoreKey()] = m
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
		return errors.New("Desired state is not fresh")
	}

	fresh, err = storecache.store.IsActualStateFresh(time)
	if err != nil {
		return err
	}
	if !fresh {
		return errors.New("Actual state is not fresh")
	}

	return nil
}
