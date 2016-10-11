package store

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/storeadapter"
)

var ActualIsNotFreshError = errors.New("Actual state is not fresh")
var AppNotFoundError = errors.New("App not found")

type Storeable interface {
	StoreKey() string
	ToJSON() []byte
}

//go:generate counterfeiter -o fakestore/fake_store.go . Store
type Store interface {
	BumpActualFreshness(timestamp time.Time) error
	RevokeActualFreshness() error

	IsActualStateFresh(time.Time) (bool, error)

	VerifyFreshness(time.Time) error

	AppKey(appGuid string, appVersion string) string
	GetApps() (map[string]*models.App, error)
	GetApp(appGuid string, appVersion string) (*models.App, error)

	SyncHeartbeats(heartbeat ...*models.Heartbeat) ([]models.InstanceHeartbeat, error)
	GetInstanceHeartbeats() (results []models.InstanceHeartbeat, err error)
	GetInstanceHeartbeatsForApp(appGuid string, appVersion string) (results []models.InstanceHeartbeat, err error)

	SaveCrashCounts(crashCounts ...models.CrashCount) error

	SavePendingStartMessages(startMessages ...models.PendingStartMessage) error
	GetPendingStartMessages() (map[string]models.PendingStartMessage, error)
	DeletePendingStartMessages(startMessages ...models.PendingStartMessage) error

	SavePendingStopMessages(stopMessages ...models.PendingStopMessage) error
	GetPendingStopMessages() (map[string]models.PendingStopMessage, error)
	DeletePendingStopMessages(stopMessages ...models.PendingStopMessage) error

	SaveMetric(metric string, value float64) error
	GetMetric(metric string) (float64, error)

	Compact() error

	GetDeaCache() (map[string]struct{}, error)
}

type RealStore struct {
	config  *config.Config
	adapter storeadapter.StoreAdapter
	logger  lager.Logger

	instanceHeartbeatCache          map[string]models.InstanceHeartbeat
	instanceHeartbeatCacheMutex     *sync.Mutex
	instanceHeartbeatCacheTimestamp time.Time

	deaLock                     *sync.Mutex
	deaCache                    map[string]struct{}
	deaCacheExperationTimestamp time.Time
}

func NewStore(config *config.Config, adapter storeadapter.StoreAdapter, logger lager.Logger) *RealStore {
	return &RealStore{
		config:                          config,
		adapter:                         adapter,
		logger:                          logger,
		instanceHeartbeatCache:          map[string]models.InstanceHeartbeat{},
		instanceHeartbeatCacheMutex:     &sync.Mutex{},
		instanceHeartbeatCacheTimestamp: time.Unix(0, 0),
		deaCache:                        map[string]struct{}{},
		deaLock:                         &sync.Mutex{},
	}
}

func (store *RealStore) SchemaRoot() string {
	return "/hm/v" + strconv.Itoa(store.config.StoreSchemaVersion)
}

func (store *RealStore) fetchNodesUnderDir(dir string) ([]storeadapter.StoreNode, error) {
	node, err := store.adapter.ListRecursively(dir)
	if err != nil {
		if err.(storeadapter.Error).Type() == storeadapter.ErrorKeyNotFound {
			return []storeadapter.StoreNode{}, nil
		}
		return []storeadapter.StoreNode{}, err
	}
	return node.ChildNodes, nil
}

func (store *RealStore) save(stuff interface{}, root string, ttl uint64) error {
	t := time.Now()
	arrValue := reflect.ValueOf(stuff)

	nodes := make([]storeadapter.StoreNode, arrValue.Len())
	for i := 0; i < arrValue.Len(); i++ {
		item := arrValue.Index(i).Interface().(Storeable)
		nodes[i] = storeadapter.StoreNode{
			Key:   root + "/" + item.StoreKey(),
			Value: item.ToJSON(),
			TTL:   ttl,
		}
	}

	err := store.adapter.SetMulti(nodes)

	store.logger.Debug(fmt.Sprintf("Save Duration %s", root), lager.Data{
		"Number of Items": fmt.Sprintf("%d", arrValue.Len()),
		"Duration":        fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
	})
	return err
}

func (store *RealStore) get(root string, mapType reflect.Type, constructor reflect.Value) (reflect.Value, error) {
	t := time.Now()

	nodes, err := store.fetchNodesUnderDir(root)
	if err != nil {
		return reflect.MakeMap(mapType), err
	}

	mapToReturn := reflect.MakeMap(mapType)
	for _, node := range nodes {
		out := constructor.Call([]reflect.Value{reflect.ValueOf(node.Value)})
		if !out[1].IsNil() {
			return reflect.MakeMap(mapType), out[1].Interface().(error)
		}
		item := out[0].Interface().(Storeable)
		mapToReturn.SetMapIndex(reflect.ValueOf(item.StoreKey()), out[0])
	}

	store.logger.Debug(fmt.Sprintf("Get Duration %s", root), lager.Data{
		"Number of Items": fmt.Sprintf("%d", mapToReturn.Len()),
		"Duration":        fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
	})
	return mapToReturn, nil
}

func (store *RealStore) delete(stuff interface{}, root string) error {
	t := time.Now()
	arrValue := reflect.ValueOf(stuff)

	keysToDelete := []string{}
	for i := 0; i < arrValue.Len(); i++ {
		item := arrValue.Index(i).Interface().(Storeable)
		keysToDelete = append(keysToDelete, root+"/"+item.StoreKey())
	}

	err := store.adapter.Delete(keysToDelete...)

	store.logger.Debug(fmt.Sprintf("Delete Duration %s", root), lager.Data{
		"Number of Items": fmt.Sprintf("%d", arrValue.Len()),
		"Duration":        fmt.Sprintf("%.4f seconds", time.Since(t).Seconds()),
	})

	return err
}
