package store

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"time"
)

func (store *RealStore) actualStateStoreKey(actualState models.InstanceHeartbeat) string {
	return "/apps/actual/" + store.AppKey(actualState.AppGuid, actualState.AppVersion) + "/" + actualState.StoreKey()
}

func (store *RealStore) SaveActualState(actualStates ...models.InstanceHeartbeat) error {
	t := time.Now()

	nodes := make([]storeadapter.StoreNode, len(actualStates))
	for i, actualState := range actualStates {
		nodes[i] = storeadapter.StoreNode{
			Key:   store.actualStateStoreKey(actualState),
			Value: actualState.ToJSON(),
			TTL:   store.config.HeartbeatTTL(),
		}
	}

	err := store.adapter.Set(nodes)

	store.logger.Debug(fmt.Sprintf("Save Duration Actual"), map[string]string{
		"Number of Items": fmt.Sprintf("%d", len(actualStates)),
		"Duration":        fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
	})
	return err
}

func (store *RealStore) getActualStates() (results []models.InstanceHeartbeat, err error) {
	node, err := store.adapter.ListRecursively("/apps/actual")

	if err == storeadapter.ErrorKeyNotFound {
		return results, nil
	} else if err != nil {
		return results, err
	}

	for _, actualNode := range node.ChildNodes {
		heartbeats, err := store.heartbeatsForNode(actualNode)
		if err != nil {
			return []models.InstanceHeartbeat{}, nil
		}
		results = append(results, heartbeats...)
	}

	return results, nil
}

func (store *RealStore) getActualStateForApp(appGuid string, appVersion string) (results []models.InstanceHeartbeat, err error) {
	node, err := store.adapter.ListRecursively("/apps/actual/" + store.AppKey(appGuid, appVersion))
	if err == storeadapter.ErrorKeyNotFound {
		return []models.InstanceHeartbeat{}, nil
	} else if err != nil {
		return []models.InstanceHeartbeat{}, err
	}

	return store.heartbeatsForNode(node)
}

func (store *RealStore) heartbeatsForNode(node storeadapter.StoreNode) (results []models.InstanceHeartbeat, err error) {
	for _, heartbeatNode := range node.ChildNodes {
		heartbeat, err := models.NewInstanceHeartbeatFromJSON(heartbeatNode.Value)
		if err != nil {
			return []models.InstanceHeartbeat{}, err
		}

		results = append(results, heartbeat)
	}
	return results, nil
}
