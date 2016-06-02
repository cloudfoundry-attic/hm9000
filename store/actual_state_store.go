package store

import (
	"fmt"
	"strings"
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

func (store *RealStore) GetStoredDeaHeartbeats() ([]storeadapter.StoreNode, error) {
	summaryNodes, err := store.adapter.ListRecursively(store.SchemaRoot() + "/dea-presence")
	if err == storeadapter.ErrorKeyNotFound {
		// nothing to do, that just means there was no key
	} else if err != nil {
		return []storeadapter.StoreNode{}, err
	}

	return summaryNodes.ChildNodes, nil
}

func (store *RealStore) GetStoredInstanceHeartbeats() (results []models.InstanceHeartbeat, err error) {
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

// Done
func (store *RealStore) heartbeatsForNode(node storeadapter.StoreNode, unexpiredDeas map[string]bool) (results []models.InstanceHeartbeat, toDelete []string, err error) {
	results = []models.InstanceHeartbeat{}
	for _, heartbeatNode := range node.ChildNodes {
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

// Done
func componentsForKey(key string) (string, string, string) {
	components := strings.Split(key, "/")
	instanceGuid := components[len(components)-1]
	appGuidVersion := strings.Split(components[len(components)-2], ",")
	return appGuidVersion[0], appGuidVersion[1], instanceGuid
}

func (store *RealStore) unexpiredDeas() (results map[string]bool, err error) {
	results = map[string]bool{}

	nodes, err := store.GetStoredDeaHeartbeats()
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
