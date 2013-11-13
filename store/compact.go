package store

import (
	"github.com/cloudfoundry/hm9000/storeadapter"
	"strings"
)

func (store *RealStore) Compact() error {
	err := store.deleteExpiredDEAHeartbeatSummaries()
	if err != nil {
		return err
	}

	err = store.deleteEmptyDirectories()
	if err != nil {
		return err
	}
	return nil
}

func (store *RealStore) deleteExpiredDEAHeartbeatSummaries() error {
	summaries, err := store.adapter.ListRecursively("/dea-summary")
	if err == storeadapter.ErrorKeyNotFound {
		return nil
	} else if err != nil {
		return err
	}

	presence, err := store.adapter.ListRecursively("/dea-presence")
	if err != nil && err != storeadapter.ErrorKeyNotFound {
		return err
	}

	presentDeaGuids := map[string]bool{}
	for _, node := range presence.ChildNodes {
		keyComponents := strings.Split(node.Key, "/")
		guid := keyComponents[len(keyComponents)-1]
		presentDeaGuids[guid] = true
	}

	keysToDelete := []string{}
	for _, node := range summaries.ChildNodes {
		keyComponents := strings.Split(node.Key, "/")
		guid := keyComponents[len(keyComponents)-1]
		if !presentDeaGuids[guid] {
			keysToDelete = append(keysToDelete, node.Key)
		}
	}

	return store.adapter.Delete(keysToDelete...)
}

func (store *RealStore) deleteEmptyDirectories() error {
	node, err := store.adapter.ListRecursively("/")
	if err != nil {
		store.logger.Error("Failed to recursively fetch /", err)
		return err
	}

	store.deleteEmptyDirectoriesUnder(node)
	return nil
}

func (store *RealStore) deleteEmptyDirectoriesUnder(node storeadapter.StoreNode) bool {
	if node.Dir {
		if len(node.ChildNodes) == 0 {
			// ignoring errors -- best effort!
			store.logger.Info("Deleting Key", map[string]string{"Key": node.Key})
			store.adapter.Delete(node.Key)
			return true
		} else {
			deletedAll := true

			for _, child := range node.ChildNodes {
				deletedAll = store.deleteEmptyDirectoriesUnder(child) && deletedAll
			}

			if deletedAll {
				// ignoring errors -- best effort!
				store.logger.Info("Deleting Key", map[string]string{"Key": node.Key})
				store.adapter.Delete(node.Key)
				return true
			}
		}
	}

	return false
}
