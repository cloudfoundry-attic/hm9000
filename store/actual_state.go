package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/storeadapter"
	"github.com/pivotal-golang/lager"
)

func (store *RealStore) EnsureCacheIsReady() error {
	store.instanceHeartbeatCacheMutex.Lock()
	defer store.instanceHeartbeatCacheMutex.Unlock()

	t := time.Now()
	heartbeats, err := store.getStoredInstanceHeartbeats()
	if err != nil {
		return err
	}

	store.instanceHeartbeatCache = map[string]instanceHeartbeat{}
	for _, heartbeat := range heartbeats {
		store.instanceHeartbeatCache[store.AppKey(heartbeat.AppGuid, heartbeat.AppVersion)] = instanceHeartbeat{heartbeat.InstanceGuid: heartbeat}
	}
	store.instanceHeartbeatCacheTimestamp = time.Now()
	store.logger.Info("Loaded actual state cache from store", lager.Data{
		"Duration":                   time.Since(t).String(),
		"Instance Heartbeats Loaded": fmt.Sprintf("%d", len(store.instanceHeartbeatCache)),
	})

	nodes, err := store.GetDeaHeartbeats()
	if err != nil {
		return nil
	}

	for _, deaPresenceNode := range nodes {
		deaKey := strings.Split(deaPresenceNode.Key, "dea-presence/")
		store.deaHeartbeatCache[deaKey[1]] = time.Now().Add(time.Duration(deaPresenceNode.TTL) * time.Second)
	}

	return nil
}

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
				store.instanceHeartbeatCache[cacheKey] = instanceHeartbeat{}
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

					// lookupKey := store.AppKey(existingInstanceHeartbeat.AppGuid, existingInstanceHeartbeat.AppVersion)
					cacheKeysToDelete = append(cacheKeysToDelete, existingInstanceHeartbeat.InstanceGuid)
					// instanceCache := store.instanceHeartbeatCache[lookupKey]
					// delete(store.instanceHeartbeatCache[lookupKey], existingInstanceHeartbeat.InstanceGuid)

				}
			}

			for _, key := range cacheKeysToDelete {
				delete(existingAppInstanceHeartbeats, key)
			}
		}
	}

	for key, val := range store.instanceHeartbeatCache {
		if len(val) == 0 {
			delete(store.instanceHeartbeatCache, key)
		}
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

// This should be called when we shutdown
func (store *RealStore) SyncCacheToStore() error {
	store.instanceHeartbeatCacheMutex.Lock()
	defer store.instanceHeartbeatCacheMutex.Unlock()
	t := time.Now()

	numberOfInstanceHeartbeats := 0

	nodesToSave := []storeadapter.StoreNode{}
	keysToDelete := []string{}

	storedHeartbeats, err := store.getStoredInstanceHeartbeats()
	if err != nil {
		return err
	}

	storedInstanceGuids := map[string]bool{}

	// Figure out what to delete...
	for _, storedHeartbeat := range storedHeartbeats {

		// Do we know about this app/version?
		if ihbs, ok := store.instanceHeartbeatCache[store.AppKey(storedHeartbeat.AppGuid, storedHeartbeat.AppVersion)]; ok {
			// Do we know about this instance?
			if _, ok := ihbs[storedHeartbeat.InstanceGuid]; !ok {
				storeKey := store.instanceHeartbeatStoreKey(storedHeartbeat.AppGuid, storedHeartbeat.AppVersion, storedHeartbeat.InstanceGuid)
				keysToDelete = append(keysToDelete, storeKey)
			} else {
				storedInstanceGuids[storedHeartbeat.InstanceGuid] = true
			}
		} else {
			storeKey := store.instanceHeartbeatStoreKey(storedHeartbeat.AppGuid, storedHeartbeat.AppVersion, storedHeartbeat.InstanceGuid)
			keysToDelete = append(keysToDelete, storeKey)
		}
	}

	// ...and delete it
	tDelete := time.Now()
	err = store.adapter.Delete(keysToDelete...)
	dtDelete := time.Since(tDelete).Seconds()

	// Figure out what to save...
	cachedDeaGuids := map[string]bool{}

	for _, appInstanceHeartbeats := range store.instanceHeartbeatCache {
		for _, cachedInstanceHeartbeat := range appInstanceHeartbeats {

			cachedDeaGuids[cachedInstanceHeartbeat.DeaGuid] = true

			if _, ok := storedInstanceGuids[cachedInstanceHeartbeat.InstanceGuid]; !ok {
				nodesToSave = append(nodesToSave, store.storeNodeForInstanceHeartbeat(cachedInstanceHeartbeat))
				numberOfInstanceHeartbeats++
			}

		}
	}

	for deaGuid, _ := range cachedDeaGuids {
		nodesToSave = append(nodesToSave, store.deaPresenceNode(deaGuid))
	}

	// ...and save it
	tSave := time.Now()
	err = store.adapter.SetMulti(nodesToSave)
	dtSave := time.Since(tSave).Seconds()

	store.logger.Debug(fmt.Sprintf("Sync Duration Actual"), lager.Data{
		"Instance Heartbeats Saved":   fmt.Sprintf("%d", numberOfInstanceHeartbeats),
		"Number of Items Saved":       fmt.Sprintf("%d", len(nodesToSave)),
		"Instance Heartbeats Deleted": fmt.Sprintf("%d", len(keysToDelete)),
		"Sync Duration":               fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
		"Save Duration":               fmt.Sprintf("%.4f seconds", dtSave),
		"Delete Duration":             fmt.Sprintf("%.4f seconds", dtDelete),
	})

	return err
}

func (store *RealStore) GetCachedInstanceHeartbeats() []models.InstanceHeartbeat {
	results := make([]models.InstanceHeartbeat, 0, len(store.instanceHeartbeatCache))

	for _, ahbs := range store.instanceHeartbeatCache {
		for _, hb := range ahbs {
			results = append(results, hb)
		}
	}

	return results
}

func (store *RealStore) GetInstanceHeartbeats() (results []models.InstanceHeartbeat, err error) {
	return store.getStoredInstanceHeartbeats()
}

// TODO TEST ME
func (store *RealStore) GetDeaHeartbeats() ([]storeadapter.StoreNode, error) {
	summaryNodes, err := store.adapter.ListRecursively(store.SchemaRoot() + "/dea-presence")
	if err == storeadapter.ErrorKeyNotFound {
		// nothing to do, that just means there was no key
	} else if err != nil {
		return []storeadapter.StoreNode{}, err
	}

	return summaryNodes.ChildNodes, nil
}

func (store *RealStore) getStoredInstanceHeartbeats() (results []models.InstanceHeartbeat, err error) {
	results = []models.InstanceHeartbeat{}
	node, err := store.adapter.ListRecursively(store.SchemaRoot() + "/apps/actual")
	if err == storeadapter.ErrorKeyNotFound {
		return results, nil
	} else if err != nil {
		return results, err
	}

	unexpiredDeas, err := store.unexpiredDeas()
	if err != nil {
		return results, err
	}

	expiredKeys := []string{}
	for _, actualNode := range node.ChildNodes {
		heartbeats, toDelete, err := store.heartbeatsForNode(actualNode, unexpiredDeas)
		if err != nil {
			return []models.InstanceHeartbeat{}, nil
		}
		results = append(results, heartbeats...)
		expiredKeys = append(expiredKeys, toDelete...)
	}

	err = store.adapter.Delete(expiredKeys...)
	if err == storeadapter.ErrorKeyNotFound {
		store.logger.Debug("store.GetInstanceHeartbeats Failed to delete a key, soldiering on...")
	} else if err != nil {
		return []models.InstanceHeartbeat{}, err
	}

	return results, nil
}

func (store *RealStore) GetInstanceHeartbeatsForApp(appGuid string, appVersion string) (results []models.InstanceHeartbeat, err error) {
	return store.getCachedInstanceHeartbeatsForApp(appGuid, appVersion)
}

func (store *RealStore) getCachedInstanceHeartbeatsForApp(appGuid string, appVersion string) ([]models.InstanceHeartbeat, error) {
	key := store.AppKey(appGuid, appVersion)
	results := []models.InstanceHeartbeat{}

	unexpiredDeas, err := store.unexpiredDeas()
	if err != nil {
		return results, err
	}

	for instanceGuid, hb := range store.instanceHeartbeatCache[key] {
		if unexpiredDeas[hb.DeaGuid] {
			results = append(results, hb)
		} else {
			// Delete from both cache and store
			delete(store.instanceHeartbeatCache[key], instanceGuid)
			store.adapter.Delete(store.instanceHeartbeatStoreKey(hb.AppGuid, hb.AppVersion, instanceGuid))
		}
	}

	return results, nil
}

func (store *RealStore) getStoredInstanceHeartbeatsForApp(appGuid string, appVersion string) (results []models.InstanceHeartbeat, err error) {
	node, err := store.adapter.ListRecursively(store.SchemaRoot() + "/apps/actual/" + store.AppKey(appGuid, appVersion))
	if err == storeadapter.ErrorKeyNotFound {
		return []models.InstanceHeartbeat{}, nil
	} else if err != nil {
		return []models.InstanceHeartbeat{}, err
	}

	unexpiredDeas, err := store.unexpiredDeas()
	if err != nil {
		return results, err
	}

	results, expiredKeys, err := store.heartbeatsForNode(node, unexpiredDeas)
	if err != nil {
		return []models.InstanceHeartbeat{}, err
	}

	err = store.adapter.Delete(expiredKeys...)
	if err == storeadapter.ErrorKeyNotFound {
		store.logger.Debug("store.GetInstanceHeartbeatsForApp Failed to delete a key, soldiering on...")
	} else if err != nil {
		return []models.InstanceHeartbeat{}, err
	}

	return results, nil
}

func (store *RealStore) heartbeatsForNode(node storeadapter.StoreNode, unexpiredDeas map[string]bool) (results []models.InstanceHeartbeat, toDelete []string, err error) {
	results = []models.InstanceHeartbeat{}
	for _, heartbeatNode := range node.ChildNodes {
		// components := strings.Split(heartbeatNode.Key, "/")
		// instanceGuid := components[len(components)-1]
		// appGuidVersion := strings.Split(components[len(components)-2], ",")
		appGuid, appVersion, instanceGuid := componentsForKey(heartbeatNode.Key)
		heartbeat, err := models.NewInstanceHeartbeatFromCSV(appGuid, appVersion, instanceGuid, heartbeatNode.Value)
		if err != nil {
			return []models.InstanceHeartbeat{}, []string{}, err
		}

		_, deaIsPresent := unexpiredDeas[heartbeat.DeaGuid]

		if deaIsPresent {
			results = append(results, heartbeat)
		} else {
			toDelete = append(toDelete, heartbeatNode.Key)
		}
	}

	return results, toDelete, nil
}

func componentsForKey(key string) (string, string, string) {
	components := strings.Split(key, "/")
	instanceGuid := components[len(components)-1]
	appGuidVersion := strings.Split(components[len(components)-2], ",")
	return appGuidVersion[0], appGuidVersion[1], instanceGuid
}

func (store *RealStore) unexpiredDeas() (results map[string]bool, err error) {
	results = map[string]bool{}

	nodes, err := store.GetDeaHeartbeats()
	if err != nil {
		return results, nil
	}

	for _, deaPresenceNode := range nodes {
		results[string(deaPresenceNode.Value)] = true
	}

	return results, nil
}

func (store *RealStore) instanceHeartbeatStoreKey(appGuid string, appVersion string, instanceGuid string) string {
	return store.SchemaRoot() + "/apps/actual/" + store.heartbeatKey(appGuid, appVersion, instanceGuid)
}

func (store *RealStore) heartbeatKey(appGuid string, appVersion string, instanceGuid string) string {
	return store.AppKey(appGuid, appVersion) + "/" + instanceGuid
}

func (store *RealStore) deaPresenceNode(deaGuid string) storeadapter.StoreNode {
	return storeadapter.StoreNode{
		Key:   store.SchemaRoot() + "/dea-presence/" + deaGuid,
		Value: []byte(deaGuid),
		TTL:   store.config.HeartbeatTTL(),
	}
}

func (store *RealStore) storeNodeForInstanceHeartbeat(instanceHeartbeat models.InstanceHeartbeat) storeadapter.StoreNode {
	return storeadapter.StoreNode{
		Key:   store.instanceHeartbeatStoreKey(instanceHeartbeat.AppGuid, instanceHeartbeat.AppVersion, instanceHeartbeat.InstanceGuid),
		Value: instanceHeartbeat.ToCSV(),
	}
}
