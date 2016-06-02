package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/storeadapter"
	"github.com/pivotal-golang/lager"
)

// Should be called on vm startup
func (store *RealStore) EnsureCacheIsReady() error {
	store.instanceHeartbeatCacheMutex.Lock()
	defer store.instanceHeartbeatCacheMutex.Unlock()

	t := time.Now()
	heartbeats, err := store.GetStoredInstanceHeartbeats() // use etcd
	if err != nil {
		return err
	}

	for _, heartbeat := range heartbeats {
		if store.instanceHeartbeatCache[store.AppKey(heartbeat.AppGuid, heartbeat.AppVersion)] == nil {
			store.instanceHeartbeatCache[store.AppKey(heartbeat.AppGuid, heartbeat.AppVersion)] = InstanceHeartbeats{heartbeat.InstanceGuid: heartbeat}
		} else {
			store.instanceHeartbeatCache[store.AppKey(heartbeat.AppGuid, heartbeat.AppVersion)][heartbeat.InstanceGuid] = heartbeat
		}
	}
	store.instanceHeartbeatCacheTimestamp = time.Now()
	store.logger.Info("Loaded actual state cache from store", lager.Data{
		"Duration":                   time.Since(t).String(),
		"Instance Heartbeats Loaded": fmt.Sprintf("%d", len(store.instanceHeartbeatCache)),
	})

	nodes, err := store.GetStoredDeaHeartbeats() // use etcd
	if err != nil {
		return nil
	}

	for _, deaPresenceNode := range nodes {
		deaKey := strings.Split(deaPresenceNode.Key, "dea-presence/")
		store.deaHeartbeatCache[deaKey[1]] = time.Now().Add(time.Duration(deaPresenceNode.TTL) * time.Second).UnixNano()
	}

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
