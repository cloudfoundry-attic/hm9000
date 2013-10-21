package store

import (
	"github.com/cloudfoundry/hm9000/models"
)

func (store *RealStore) AppKey(appGuid string, appVersion string) string {
	return appGuid + "-" + appVersion
}

func (store *RealStore) GetApp(appGuid string, appVersion string) (*models.App, error) {
	apps, err := store.GetApps()
	if err != nil {
		return nil, err
	}

	app, found := apps[store.AppKey(appGuid, appVersion)]
	if !found {
		return nil, AppNotFoundError
	}

	return app, nil
}

func (store *RealStore) GetApps() (map[string]*models.App, error) {
	apps := make(map[string]*models.App)

	desireds, err := store.GetDesiredState()
	if err != nil {
		return apps, err
	}

	actuals, err := store.GetActualState()
	if err != nil {
		return apps, err
	}

	crashCounts, err := store.GetCrashCounts()
	if err != nil {
		return apps, err
	}

	actualsByApp := make(map[string][]models.InstanceHeartbeat, 0)
	crashCountsLookup := make(map[string]map[int]models.CrashCount)

	for _, actual := range actuals {
		appKey := store.AppKey(actual.AppGuid, actual.AppVersion)
		actualsByApp[appKey] = append(actualsByApp[appKey], actual)
	}

	for _, crashCount := range crashCounts {
		key := store.AppKey(crashCount.AppGuid, crashCount.AppVersion)
		_, present := crashCountsLookup[key]
		if !present {
			crashCountsLookup[key] = make(map[int]models.CrashCount)
		}
		crashCountsLookup[key][crashCount.InstanceIndex] = crashCount
	}

	for key, desired := range desireds {
		apps[key] = models.NewApp(desired.AppGuid, desired.AppVersion, desired, actualsByApp[key], crashCountsLookup[key])
	}

	for key, instances := range actualsByApp {
		_, present := apps[key]
		if !present {
			apps[key] = models.NewApp(instances[0].AppGuid, instances[0].AppVersion, models.DesiredAppState{}, instances, crashCountsLookup[key])
		}
	}

	return apps, nil
}
