package store

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"time"
)

func (store *RealStore) desiredStateStoreKey(desiredState models.DesiredAppState) string {
	return "/apps/" + store.AppKey(desiredState.AppGuid, desiredState.AppVersion) + "/desired"
}

func (store *RealStore) SyncDesiredState(newDesiredStates ...models.DesiredAppState) error {
	t := time.Now()

	currentDesiredStates, err := store.GetDesiredState()
	if err != nil {
		return err
	}

	newDesiredStateKeys := make(map[string]bool, 0)
	nodesToSave := make([]storeadapter.StoreNode, 0)
	for _, newDesiredState := range newDesiredStates {
		key := newDesiredState.StoreKey()
		newDesiredStateKeys[key] = true

		currentDesiredState, present := currentDesiredStates[key]
		if !(present && newDesiredState.Equal(currentDesiredState)) {
			nodesToSave = append(nodesToSave, storeadapter.StoreNode{
				Key:   store.desiredStateStoreKey(newDesiredState),
				Value: newDesiredState.ToJSON(),
			})
		}
	}

	err = store.adapter.Set(nodesToSave)
	if err != nil {
		return err
	}

	numberOfDeletedNodes := 0
	for key, currentDesiredState := range currentDesiredStates {
		if !newDesiredStateKeys[key] {
			err = store.adapter.Delete(store.desiredStateStoreKey(currentDesiredState))
			numberOfDeletedNodes += 1
		}
	}

	store.logger.Debug(fmt.Sprintf("Save Duration Desired"), map[string]string{
		"Number of Items Synced":  fmt.Sprintf("%d", len(newDesiredStates)),
		"Number of Items Saved":   fmt.Sprintf("%d", len(nodesToSave)),
		"Number of Items Deleted": fmt.Sprintf("%d", numberOfDeletedNodes),
		"Duration":                fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
	})
	return err
}

func (store *RealStore) GetDesiredState() (results map[string]models.DesiredAppState, err error) {
	t := time.Now()

	results = make(map[string]models.DesiredAppState)

	apps, err := store.GetApps()
	if err != nil {
		return results, err
	}

	for _, app := range apps {
		if app.Desired.AppGuid != "" {
			results[app.Desired.StoreKey()] = app.Desired
		}
	}

	store.logger.Debug(fmt.Sprintf("Get Duration Desired"), map[string]string{
		"Number of Items": fmt.Sprintf("%d", len(results)),
		"Duration":        fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
	})
	return results, nil
}
