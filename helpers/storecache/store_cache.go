package storecache

import (
	"errors"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"strconv"
	"time"
)

type StoreCache struct {
	store store.Store

	DesiredStates []models.DesiredAppState
	ActualStates  []models.InstanceHeartbeat
	CrashCounts   []models.CrashCount

	SetOfApps                      map[string]bool
	HeartbeatingInstancesByApp     map[string][]models.InstanceHeartbeat
	DesiredByApp                   map[string]models.DesiredAppState
	HeartbeatingInstancesByGuid    map[string]models.InstanceHeartbeat
	crashCountByAppVersionIndexKey map[string]models.CrashCount
}

func New(store store.Store) (storecache *StoreCache) {
	return &StoreCache{
		store:                       store,
		DesiredStates:               make([]models.DesiredAppState, 0),
		ActualStates:                make([]models.InstanceHeartbeat, 0),
		SetOfApps:                   make(map[string]bool, 0),
		HeartbeatingInstancesByApp:  make(map[string][]models.InstanceHeartbeat, 0),
		DesiredByApp:                make(map[string]models.DesiredAppState, 0),
		HeartbeatingInstancesByGuid: make(map[string]models.InstanceHeartbeat, 0),
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

	storecache.SetOfApps = make(map[string]bool, 0)
	storecache.HeartbeatingInstancesByApp = make(map[string][]models.InstanceHeartbeat, 0)
	storecache.DesiredByApp = make(map[string]models.DesiredAppState, 0)
	storecache.HeartbeatingInstancesByGuid = make(map[string]models.InstanceHeartbeat, 0)
	storecache.crashCountByAppVersionIndexKey = make(map[string]models.CrashCount, 0)

	for _, desired := range storecache.DesiredStates {
		appKey := storecache.Key(desired.AppGuid, desired.AppVersion)
		storecache.DesiredByApp[appKey] = desired
		storecache.SetOfApps[appKey] = true
	}

	for _, actual := range storecache.ActualStates {
		appKey := storecache.Key(actual.AppGuid, actual.AppVersion)

		storecache.HeartbeatingInstancesByGuid[actual.InstanceGuid] = actual
		storecache.HeartbeatingInstancesByApp[appKey] = append(storecache.HeartbeatingInstancesByApp[appKey], actual)
		storecache.SetOfApps[appKey] = true
	}

	for _, crashCount := range storecache.CrashCounts {
		key := crashCount.StoreKey()
		storecache.crashCountByAppVersionIndexKey[key] = crashCount
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

func (storecache *StoreCache) CrashCount(appGuid string, appVersion string, instanceIndex int) models.CrashCount {
	return storecache.crashCountByAppVersionIndexKey[appGuid+"-"+appVersion+"-"+strconv.Itoa(instanceIndex)]
}
