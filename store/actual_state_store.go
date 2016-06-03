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
