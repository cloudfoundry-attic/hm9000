package store

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"time"
)

func (store *RealStore) desiredStateStoreKey(desiredState models.DesiredAppState) string {
	return "/apps/desired/" + store.AppKey(desiredState.AppGuid, desiredState.AppVersion)
}

func (store *RealStore) SyncDesiredState(newDesiredStates ...models.DesiredAppState) error {
	t := time.Now()

	currentDesiredStates, err := store.GetDesiredState()
	if err != nil {
		return err
	}

	dtGet := time.Since(t).Seconds()

	t = time.Now()

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

	dtSet := time.Since(t).Seconds()
	t = time.Now()

	numberOfDeletedNodes := 0
	for key, currentDesiredState := range currentDesiredStates {
		if !newDesiredStateKeys[key] {
			err = store.adapter.Delete(store.desiredStateStoreKey(currentDesiredState))
			numberOfDeletedNodes += 1
		}
	}

	dtDelete := time.Since(t).Seconds()

	store.logger.Debug(fmt.Sprintf("Save Duration Desired"), map[string]string{
		"Number of Items Synced":  fmt.Sprintf("%d", len(newDesiredStates)),
		"Number of Items Saved":   fmt.Sprintf("%d", len(nodesToSave)),
		"Number of Items Deleted": fmt.Sprintf("%d", numberOfDeletedNodes),
		"Get Duration":            fmt.Sprintf("%.4f seconds", dtGet),
		"Set Duration":            fmt.Sprintf("%.4f seconds", dtSet),
		"Delete Duration":         fmt.Sprintf("%.4f seconds", dtDelete),
	})
	return err
}

func (store *RealStore) GetDesiredState() (results map[string]models.DesiredAppState, err error) {
	t := time.Now()

	results = make(map[string]models.DesiredAppState)

	node, err := store.adapter.ListRecursively("/apps/desired")

	if err == storeadapter.ErrorKeyNotFound {
		return results, nil
	} else if err != nil {
		return results, err
	}

	for _, desiredNode := range node.ChildNodes {
		desiredState, err := models.NewDesiredAppStateFromJSON(desiredNode.Value)
		if err != nil {
			return results, err
		}

		results[desiredState.StoreKey()] = desiredState
	}

	store.logger.Debug(fmt.Sprintf("Get Duration Desired"), map[string]string{
		"Number of Items": fmt.Sprintf("%d", len(results)),
		"Duration":        fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
	})
	return results, nil
}

func (store *RealStore) getDesiredStateForApp(appGuid string, appVersion string) (desired models.DesiredAppState, err error) {
	node, err := store.adapter.Get("/apps/desired/" + store.AppKey(appGuid, appVersion))
	if err == storeadapter.ErrorKeyNotFound {
		return desired, nil
	} else if err != nil {
		return desired, err
	}

	return models.NewDesiredAppStateFromJSON(node.Value)
}
