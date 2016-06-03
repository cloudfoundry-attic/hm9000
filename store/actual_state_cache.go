package store

import (
	"fmt"
	"time"

	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/storeadapter"
	"github.com/pivotal-golang/lager"
)

// Done? Think we should delete old heartbeats here
// make sure that if we have something in the cache and we receive a heartbeat that does not have something in our cache we need to delete it.
func (store *RealStore) SyncHeartbeats(incomingHeartbeats ...*models.Heartbeat) error {
	t := time.Now()
	var err error

	deaNodeGuids := []string{}
	nodesToSave := []storeadapter.StoreNode{}
	keysToDelete := []string{}
	numberOfInstanceHeartbeats := 0

	store.instanceHeartbeatCacheMutex.Lock()

	for _, incomingHeartbeat := range incomingHeartbeats {

		numberOfInstanceHeartbeats += len(incomingHeartbeat.InstanceHeartbeats)
		incomingInstanceGuids := map[string]bool{}
		nodesToSave = append(nodesToSave, store.deaPresenceNode(incomingHeartbeat.DeaGuid))
		deaNodeGuids = append(deaNodeGuids, incomingHeartbeat.DeaGuid)

		// Build a list of the instances for all instances in a dea's heartbeat
		for _, incomingInstanceHeartbeat := range incomingHeartbeat.InstanceHeartbeats {
			incomingInstanceGuids[incomingInstanceHeartbeat.InstanceGuid] = true

			nodesToSave = append(nodesToSave, store.storeNodeForInstanceHeartbeat(incomingInstanceHeartbeat))

			cacheKey := store.AppKey(incomingInstanceHeartbeat.AppGuid, incomingInstanceHeartbeat.AppVersion)
			if store.instanceHeartbeatCache[cacheKey] == nil {
				store.instanceHeartbeatCache[cacheKey] = InstanceHeartbeats{}
			}

			store.instanceHeartbeatCache[cacheKey][incomingInstanceHeartbeat.InstanceGuid] = incomingInstanceHeartbeat
		}

		// Get instance heartbeats for the appGuid, appVersion tuple
		for _, existingAppInstanceHeartbeats := range store.instanceHeartbeatCache {

			cacheKeysToDelete := []string{}

			// Iterate over individual instance heartbeats
			for _, existingInstanceHeartbeat := range existingAppInstanceHeartbeats {
				if existingInstanceHeartbeat.DeaGuid == incomingHeartbeat.DeaGuid && !incomingInstanceGuids[existingInstanceHeartbeat.InstanceGuid] {
					// Schedule for deletion from store
					storeKey := store.instanceHeartbeatStoreKey(existingInstanceHeartbeat.AppGuid, existingInstanceHeartbeat.AppVersion, existingInstanceHeartbeat.InstanceGuid)
					keysToDelete = append(keysToDelete, storeKey)

					cacheKeysToDelete = append(cacheKeysToDelete, existingInstanceHeartbeat.InstanceGuid)
				}
			}

			for _, key := range cacheKeysToDelete {
				delete(existingAppInstanceHeartbeats, key)
			}
		}
	}

	cacheKeysToDelete := []string{}
	for key, val := range store.instanceHeartbeatCache {
		if len(val) == 0 {
			cacheKeysToDelete = append(cacheKeysToDelete, key)
		}
	}

	for _, key := range cacheKeysToDelete {
		delete(store.instanceHeartbeatCache, key)
	}

	store.instanceHeartbeatCacheMutex.Unlock()

	tSave := time.Now()
	err = store.adapter.SetMulti(nodesToSave)
	dtSave := time.Since(tSave).Seconds()

	if err != nil {
		return err
	}

	store.AddDeaHeartbeats(deaNodeGuids)

	tDelete := time.Now()
	err = store.adapter.Delete(keysToDelete...)
	dtDelete := time.Since(tDelete).Seconds()

	if err == storeadapter.ErrorKeyNotFound {
		store.logger.Debug("store.SyncHeartbeats Failed to delete a key, soldiering on...")
	} else if err != nil {
		return err
	}

	store.logger.Debug(fmt.Sprintf("Save Duration Actual"), lager.Data{
		"Number of Heartbeats":          fmt.Sprintf("%d", len(incomingHeartbeats)),
		"Number of Instance Heartbeats": fmt.Sprintf("%d", numberOfInstanceHeartbeats),
		"Number of Items Saved":         fmt.Sprintf("%d", len(nodesToSave)),
		"Number of Items Deleted":       fmt.Sprintf("%d", len(keysToDelete)),
		"Duration":                      fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
		"Save Duration":                 fmt.Sprintf("%.4f seconds", dtSave),
		"Delete Duration":               fmt.Sprintf("%.4f seconds", dtDelete),
	})

	return nil
}

func (store *RealStore) GetCachedInstanceHeartbeats() ([]models.InstanceHeartbeat, error) {
	results := make([]models.InstanceHeartbeat, 0, len(store.instanceHeartbeatCache))

	cachedDeaHeartbeats := store.GetCachedDeaHeartbeats()
	lookupTime := time.Now().UnixNano()

	// StoreKey -> Instance
	instancesToDelete := make(map[string]models.InstanceHeartbeat)

	for _, appInstanceHeartbeats := range store.instanceHeartbeatCache {
		for _, appInstance := range appInstanceHeartbeats {
			if len(cachedDeaHeartbeats) == 0 {
				instancesToDelete[store.instanceHeartbeatStoreKey(appInstance.AppGuid, appInstance.AppVersion, appInstance.InstanceGuid)] = appInstance
				continue
			}

			if lookupTime <= cachedDeaHeartbeats[appInstance.DeaGuid] {
				results = append(results, appInstance)
			} else {
				instancesToDelete[store.instanceHeartbeatStoreKey(appInstance.AppGuid, appInstance.AppVersion, appInstance.InstanceGuid)] = appInstance
			}
		}
	}

	for storeKey, appInstance := range instancesToDelete {
		err := store.adapter.Delete(storeKey)
		if err == storeadapter.ErrorKeyNotFound || err == nil {
			delete(store.instanceHeartbeatCache[store.AppKey(appInstance.AppGuid, appInstance.AppVersion)], appInstance.InstanceGuid)
			if len(store.instanceHeartbeatCache[store.AppKey(appInstance.AppGuid, appInstance.AppVersion)]) == 0 {
				delete(store.instanceHeartbeatCache, store.AppKey(appInstance.AppGuid, appInstance.AppVersion))
			}
			// Continue on. We somehow have an extra cache key and it should be deleted
		} else if err != nil {
			return results, err
		}
	}

	return results, nil
}

func (store *RealStore) GetCachedInstanceHeartbeatsForApp(appGuid string, appVersion string) ([]models.InstanceHeartbeat, error) {
	results := []models.InstanceHeartbeat{}

	key := store.AppKey(appGuid, appVersion)
	cachedDeaHeartbeats := store.GetCachedDeaHeartbeats()

	//instanceGuid => instanceHeartbeat
	instancesToDelete := make(map[string]models.InstanceHeartbeat)

	for instanceGuid, hb := range store.instanceHeartbeatCache[key] {
		if cachedDeaHeartbeats[hb.DeaGuid] != 0 {
			results = append(results, hb)
		} else {
			instancesToDelete[instanceGuid] = hb
		}
	}

	for instanceGuid, instanceHeartbeat := range instancesToDelete {
		err := store.adapter.Delete(store.instanceHeartbeatStoreKey(instanceHeartbeat.AppGuid, instanceHeartbeat.AppVersion, instanceGuid))
		if err == nil || err == storeadapter.ErrorKeyNotFound {
			delete(store.instanceHeartbeatCache[key], instanceGuid)
			if len(store.instanceHeartbeatCache[key]) == 0 {
				delete(store.instanceHeartbeatCache, key)
			}
		} else {
			return results, err
		}
	}
	return results, nil
}

// return map[string]bool
func (store *RealStore) GetCachedDeaHeartbeats() map[string]int64 {
	toDelete := []string{}

	lookupTime := time.Now().UnixNano()

	for deaGuid, deaHeartbeat := range store.deaHeartbeatCache {
		if lookupTime > deaHeartbeat {
			toDelete = append(toDelete, deaGuid)
		}
	}

	for _, deaGuid := range toDelete {
		delete(store.deaHeartbeatCache, deaGuid)
	}

	return store.deaHeartbeatCache
}

func (store *RealStore) AddDeaHeartbeats(deaHeartbeatGuids []string) {
	for _, deaGuid := range deaHeartbeatGuids {
		store.deaHeartbeatCache[deaGuid] = time.Now().Add(time.Duration(store.config.HeartbeatTTL()) * time.Second).UnixNano()
	}
}
