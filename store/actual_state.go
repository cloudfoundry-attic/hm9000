package store

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"reflect"
	"time"
)

func (store *RealStore) SaveHeartbeat(newHeartbeat models.Heartbeat) error {
	t := time.Now()

	newHeartbeatSummary := newHeartbeat.HeartbeatSummary()
	existingHeartbeatSummary, err := store.getExistingHeartbeatSummary(newHeartbeat.DeaGuid)
	if err != nil {
		return err
	}

	nodesToSave := []storeadapter.StoreNode{}
	nodesToSave = append(nodesToSave, store.deaPresenceNode(newHeartbeat.DeaGuid))

	if reflect.DeepEqual(existingHeartbeatSummary, newHeartbeatSummary) {
		//no changes!
		err := store.adapter.Set(nodesToSave)

		store.logger.Debug(fmt.Sprintf("Save Duration Actual - Store is already up-to-date"), map[string]string{
			"Duration": fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
		})

		return err
	}

	nodesToSave = append(nodesToSave, store.deaSummaryNode(newHeartbeatSummary))

	//sync two sets:
	newInstanceHeartbeatGuids := map[string]bool{}
	for _, instanceHeartbeat := range newHeartbeat.InstanceHeartbeats {
		newInstanceHeartbeatGuids[instanceHeartbeat.InstanceGuid] = true

		if !existingHeartbeatSummary.ContainsInstanceHeartbeat(instanceHeartbeat) {
			nodesToSave = append(nodesToSave, store.storeNodeForInstanceHeartbeat(instanceHeartbeat))
		}
	}

	tSave := time.Now()
	err = store.adapter.Set(nodesToSave)
	dtSave := time.Since(tSave).Seconds()

	if err != nil {
		return err
	}

	keysToDelete := []string{}
	for instanceGuid, existingInstanceHeartbeatSummary := range existingHeartbeatSummary.InstanceHeartbeatSummaries {
		_, stillPresent := newInstanceHeartbeatGuids[instanceGuid]
		if !stillPresent {
			keysToDelete = append(keysToDelete, store.instanceHeartbeatStoreKey(existingInstanceHeartbeatSummary.AppGuid, existingInstanceHeartbeatSummary.AppVersion, instanceGuid))
		}
	}

	tDelete := time.Now()
	err = store.adapter.Delete(keysToDelete...)
	dtDelete := time.Since(tDelete).Seconds()

	if err != nil {
		return err
	}

	store.logger.Debug(fmt.Sprintf("Save Duration Actual"), map[string]string{
		"Number of Items":         fmt.Sprintf("%d", len(newHeartbeat.InstanceHeartbeats)),
		"Number of Items Saved":   fmt.Sprintf("%d", len(nodesToSave)),
		"Number of Items Deleted": fmt.Sprintf("%d", len(keysToDelete)),
		"Duration":                fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
		"Save Duration":           fmt.Sprintf("%.4f seconds", dtSave),
		"Delete Duration":         fmt.Sprintf("%.4f seconds", dtDelete),
	})

	return nil
}

func (store *RealStore) GetInstanceHeartbeats() (results []models.InstanceHeartbeat, err error) {
	node, err := store.adapter.ListRecursively("/apps/actual")
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
	return results, err
}

func (store *RealStore) GetInstanceHeartbeatsForApp(appGuid string, appVersion string) (results []models.InstanceHeartbeat, err error) {
	node, err := store.adapter.ListRecursively("/apps/actual/" + store.AppKey(appGuid, appVersion))
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
	return results, err
}

func (store *RealStore) heartbeatsForNode(node storeadapter.StoreNode, unexpiredDeas map[string]bool) (results []models.InstanceHeartbeat, toDelete []string, err error) {
	for _, heartbeatNode := range node.ChildNodes {
		heartbeat, err := models.NewInstanceHeartbeatFromJSON(heartbeatNode.Value)
		if err != nil {
			return []models.InstanceHeartbeat{}, []string{}, err
		}

		_, deaIsPresent := unexpiredDeas[heartbeat.DeaGuid]

		if deaIsPresent {
			results = append(results, heartbeat)
		} else {
			toDelete = append(toDelete, node.Key)
		}
	}
	return results, toDelete, nil
}

func (store *RealStore) unexpiredDeas() (results map[string]bool, err error) {
	results = map[string]bool{}

	summaryNodes, err := store.adapter.ListRecursively("/dea-presence")
	if err == storeadapter.ErrorKeyNotFound {
		return results, nil
	} else if err != nil {
		return results, err
	}

	for _, deaPresenceNode := range summaryNodes.ChildNodes {
		results[string(deaPresenceNode.Value)] = true
	}

	return results, nil
}

func (store *RealStore) instanceHeartbeatStoreKey(appGuid string, appVersion string, instanceGuid string) string {
	return "/apps/actual/" + store.AppKey(appGuid, appVersion) + "/" + instanceGuid
}

func (store *RealStore) getExistingHeartbeatSummary(deaGuid string) (models.HeartbeatSummary, error) {
	existingSummary := models.HeartbeatSummary{}
	deaSummaryNode, err := store.adapter.Get("/dea-summary/" + deaGuid)
	if err == storeadapter.ErrorKeyNotFound {
		return existingSummary, nil
	} else if err != nil {
		return existingSummary, err
	}

	existingSummary, err = models.NewHeartbeatSummaryFromJSON(deaSummaryNode.Value)
	if err != nil {
		existingSummary = models.HeartbeatSummary{}
	}

	return existingSummary, nil
}

func (store *RealStore) deaPresenceNode(deaGuid string) storeadapter.StoreNode {
	return storeadapter.StoreNode{
		Key:   "/dea-presence/" + deaGuid,
		Value: []byte(deaGuid),
		TTL:   store.config.HeartbeatTTL(),
	}
}

func (store *RealStore) deaSummaryNode(deaSummary models.HeartbeatSummary) storeadapter.StoreNode {
	return storeadapter.StoreNode{
		Key:   "/dea-summary/" + deaSummary.DeaGuid,
		Value: deaSummary.ToJSON(),
	}
}

func (store *RealStore) storeNodeForInstanceHeartbeat(instanceHeartbeat models.InstanceHeartbeat) storeadapter.StoreNode {
	return storeadapter.StoreNode{
		Key:   store.instanceHeartbeatStoreKey(instanceHeartbeat.AppGuid, instanceHeartbeat.AppVersion, instanceHeartbeat.InstanceGuid),
		Value: instanceHeartbeat.ToJSON(),
	}
}
