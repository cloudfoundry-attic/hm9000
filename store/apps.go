package store

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"strings"
	"time"
)

type appNodes struct {
	desiredNode storeadapter.StoreNode
	hasDesired  bool
	actualNode  storeadapter.StoreNode
	hasActual   bool
	crashNode   storeadapter.StoreNode
	hasCrashes  bool
}

func (store *RealStore) AppKey(appGuid string, appVersion string) string {
	return appGuid + "-" + appVersion
}

func (store *RealStore) GetApp(appGuid string, appVersion string) (*models.App, error) {
	t := time.Now()
	key := store.AppKey(appGuid, appVersion)

	nodes := &appNodes{
		hasDesired: true,
		hasActual:  true,
		hasCrashes: true,
	}

	var err error

	nodes.desiredNode, err = store.adapter.Get("/apps/desired/" + key)
	if err == storeadapter.ErrorKeyNotFound {
		nodes.hasDesired = false
	} else if err != nil {
		return nil, err
	}

	nodes.actualNode, err = store.adapter.ListRecursively("/apps/actual/" + key)
	if err == storeadapter.ErrorKeyNotFound {
		nodes.hasActual = false
	} else if err != nil {
		return nil, err
	} else if len(nodes.actualNode.ChildNodes) == 0 {
		nodes.hasActual = false
	}

	if !nodes.hasDesired && !nodes.hasActual {
		return nil, AppNotFoundError
	}

	nodes.crashNode, err = store.adapter.ListRecursively("/apps/crashes/" + key)
	if err == storeadapter.ErrorKeyNotFound {
		nodes.hasCrashes = false
	} else if err != nil {
		return nil, err
	}

	app, err := store.nodesToApp(nodes)
	if app == nil {
		return nil, AppNotFoundError
	}

	store.logger.Debug(fmt.Sprintf("Get Duration App"), map[string]string{
		"Duration": fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
	})

	return app, err
}

func (store *RealStore) GetApps() (results map[string]*models.App, err error) {
	t := time.Now()

	results = make(map[string]*models.App)
	nodes := make(map[string]*appNodes)

	appsNode, err := store.adapter.ListRecursively("/apps")

	if err == storeadapter.ErrorKeyNotFound {
		return results, nil
	} else if err != nil {
		return results, err
	}

	for _, subNode := range appsNode.ChildNodes {
		if strings.HasSuffix(subNode.Key, "desired") {
			for _, desiredNode := range subNode.ChildNodes {
				appNodes := store.appNodesForKey(desiredNode.Key, nodes)
				appNodes.hasDesired = true
				appNodes.desiredNode = desiredNode
			}
		} else if strings.HasSuffix(subNode.Key, "actual") {
			for _, actualNode := range subNode.ChildNodes {
				if len(actualNode.ChildNodes) > 0 {
					appNodes := store.appNodesForKey(actualNode.Key, nodes)
					appNodes.hasActual = true
					appNodes.actualNode = actualNode
				}
			}
		} else if strings.HasSuffix(subNode.Key, "crashes") {
			for _, crashNode := range subNode.ChildNodes {
				if len(crashNode.ChildNodes) > 0 {
					appNodes := store.appNodesForKey(crashNode.Key, nodes)
					appNodes.hasCrashes = true
					appNodes.crashNode = crashNode
				}
			}
		}
	}

	for _, appNodes := range nodes {
		if appNodes.hasDesired || appNodes.hasActual {
			app, err := store.nodesToApp(appNodes)
			if err != nil {
				return make(map[string]*models.App), err
			}
			if app != nil {
				results[store.AppKey(app.AppGuid, app.AppVersion)] = app
			}
		}
	}

	store.logger.Debug(fmt.Sprintf("Get Duration Apps"), map[string]string{
		"Number of Items": fmt.Sprintf("%d", len(results)),
		"Duration":        fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
	})

	return results, nil
}

func (store *RealStore) appNodesForKey(key string, nodes map[string]*appNodes) *appNodes {
	splitKeys := strings.Split(key, "/")
	id := splitKeys[len(splitKeys)-1]
	_, exists := nodes[id]
	if !exists {
		nodes[id] = &appNodes{}
	}
	return nodes[id]
}

func (store *RealStore) nodesToApp(nodes *appNodes) (*models.App, error) {
	desiredState := models.DesiredAppState{}
	actualState := []models.InstanceHeartbeat{}
	crashCounts := make(map[int]models.CrashCount)

	appGuid := ""
	appVersion := ""

	if nodes.hasDesired {
		desired, err := models.NewDesiredAppStateFromJSON(nodes.desiredNode.Value)
		if err != nil {
			return nil, err
		}
		desiredState = desired
		appGuid = desired.AppGuid
		appVersion = desired.AppVersion
	}

	if nodes.hasActual {
		for _, actualNode := range nodes.actualNode.ChildNodes {
			actual, err := models.NewInstanceHeartbeatFromJSON(actualNode.Value)
			if err != nil {
				return nil, err
			}
			actualState = append(actualState, actual)
			appGuid = actual.AppGuid
			appVersion = actual.AppVersion
		}
	}

	if nodes.hasCrashes {
		for _, crashNode := range nodes.crashNode.ChildNodes {
			crashCount, err := models.NewCrashCountFromJSON(crashNode.Value)
			if err != nil {
				return nil, err
			}
			crashCounts[crashCount.InstanceIndex] = crashCount
		}
	}

	if appGuid == "" || appVersion == "" {
		return nil, nil
	}

	return models.NewApp(appGuid, appVersion, desiredState, actualState, crashCounts), nil
}
