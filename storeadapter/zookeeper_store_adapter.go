package storeadapter

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/helpers/workerpool"
	"github.com/samuel/go-zookeeper/zk"
	"math"
	"strconv"

	"path"
	"strings"
	"time"
)

type ZookeeperStoreAdapter struct {
	urls              []string
	client            *zk.Conn
	workerPool        *workerpool.WorkerPool
	timeProvider      timeprovider.TimeProvider
	connectionTimeout time.Duration
}

func NewZookeeperStoreAdapter(urls []string, maxConcurrentRequests int, timeProvider timeprovider.TimeProvider, connectionTimeout time.Duration) *ZookeeperStoreAdapter {
	return &ZookeeperStoreAdapter{
		urls:              urls,
		workerPool:        workerpool.NewWorkerPool(maxConcurrentRequests),
		timeProvider:      timeProvider,
		connectionTimeout: connectionTimeout,
	}
}

func (adapter *ZookeeperStoreAdapter) Connect() error {
	var err error
	adapter.client, _, err = zk.Connect(adapter.urls, adapter.connectionTimeout)
	return err
}

func (adapter *ZookeeperStoreAdapter) Disconnect() error {
	adapter.workerPool.StopWorkers()
	adapter.client.Close()

	return nil
}

func (adapter *ZookeeperStoreAdapter) Set(nodes []StoreNode) error {
	results := make(chan error, len(nodes))
	for _, node := range nodes {
		node := node
		adapter.workerPool.ScheduleWork(func() {
			var err error

			exists, stat, err := adapter.client.Exists(node.Key)
			if err != nil {
				results <- err
				return
			}
			if stat.NumChildren > 0 {
				results <- ErrorNodeIsDirectory
				return
			}

			if exists {
				_, err = adapter.client.Set(node.Key, adapter.encode(node.Value, node.TTL), -1)
			} else {
				err = adapter.createNode(node)
			}

			results <- err
		})
	}

	var err error
	numReceived := 0
	for numReceived < len(nodes) {
		result := <-results
		numReceived++
		if err == nil {
			err = result
		}
	}

	if adapter.isTimeoutError(err) {
		return ErrorTimeout
	}

	return err
}

func (adapter *ZookeeperStoreAdapter) Get(key string) (StoreNode, error) {
	node, err := adapter.getWithTTLPolicy(key)

	if err != nil {
		return StoreNode{}, err
	}

	if node.Dir {
		return StoreNode{}, ErrorNodeIsDirectory
	}

	return node, nil
}

type nodeErrPair struct {
	node StoreNode
	err  error
}

func (adapter *ZookeeperStoreAdapter) List(key string) ([]StoreNode, error) {
	nodeKeys, _, err := adapter.client.Children(key)

	if adapter.isTimeoutError(err) {
		return []StoreNode{}, ErrorTimeout
	}

	if adapter.isMissingKeyError(err) {
		return []StoreNode{}, ErrorKeyNotFound
	}

	if err != nil {
		return []StoreNode{}, err
	}

	if len(nodeKeys) == 0 {
		return []StoreNode{}, adapter.assertNodeIsDirectory(key)
	}

	results := make(chan nodeErrPair, len(nodeKeys))
	for _, nodeKey := range nodeKeys {
		nodeKey := nodeKey
		adapter.workerPool.ScheduleWork(func() {
			node, err := adapter.getWithTTLPolicy(key + "/" + nodeKey)
			results <- nodeErrPair{node: node, err: err}
		})
	}

	nodes := make([]StoreNode, 0)
	numReceived := 0
	for numReceived < len(nodeKeys) {
		result := <-results
		numReceived++
		if result.err == nil {
			nodes = append(nodes, result.node)
		} else if result.err != ErrorKeyNotFound { //we skip KeyNotFound since it corresponds to an expired node
			if err == nil {
				err = result.err
			}
		}
	}

	if err != nil {
		return []StoreNode{}, err
	}

	return nodes, nil
}

func (adapter *ZookeeperStoreAdapter) Delete(key string) error {
	exists, stat, err := adapter.client.Exists(key)
	if adapter.isTimeoutError(err) {
		return ErrorTimeout
	}

	if err != nil {
		return err
	}

	if !exists {
		return ErrorKeyNotFound
	}

	if stat.NumChildren > 0 {
		return ErrorNodeIsDirectory
	}

	return adapter.client.Delete(key, -1)
}

func (adapter *ZookeeperStoreAdapter) isMissingKeyError(err error) bool {
	return err == zk.ErrNoNode
}

func (adapter *ZookeeperStoreAdapter) isTimeoutError(err error) bool {
	return err == zk.ErrConnectionClosed
}

func (adapter *ZookeeperStoreAdapter) encode(data []byte, TTL uint64) []byte {
	return []byte(fmt.Sprintf("%d,%s", TTL, string(data)))
}

func (adapter *ZookeeperStoreAdapter) decode(input []byte) (data []byte, TTL uint64, err error) {
	arr := strings.SplitN(string(input), ",", 2)
	if len(arr) != 2 {
		return []byte{}, 0, fmt.Errorf("Expected an encoded string of the form TTL,data, got %s", string(input))
	}
	TTL, err = strconv.ParseUint(arr[0], 10, 64)
	return []byte(arr[1]), TTL, err
}

func (adapter *ZookeeperStoreAdapter) getWithTTLPolicy(key string) (StoreNode, error) {
	data, stat, err := adapter.client.Get(key)

	if adapter.isTimeoutError(err) {
		return StoreNode{}, ErrorTimeout
	}

	if adapter.isMissingKeyError(err) {
		return StoreNode{}, ErrorKeyNotFound
	}

	if err != nil {
		return StoreNode{}, err
	}

	if len(data) == 0 {
		return StoreNode{
			Key:   key,
			Value: data,
			TTL:   0,
			Dir:   true,
		}, nil
	}

	value, TTL, err := adapter.decode(data)
	if err != nil {
		return StoreNode{}, ErrorInvalidFormat
	}

	if TTL > 0 {
		creationTime := time.Unix(int64(float64(stat.Ctime)/1000.0), 0)
		elapsedTime := int64(math.Floor(adapter.timeProvider.Time().Sub(creationTime).Seconds()))
		remainingTTL := int64(TTL) - elapsedTime
		if remainingTTL > 0 {
			if remainingTTL < int64(TTL) {
				TTL = uint64(remainingTTL)
			}
		} else {
			adapter.client.Delete(key, -1)
			return StoreNode{}, ErrorKeyNotFound
		}
	}

	return StoreNode{
		Key:   key,
		Value: value,
		TTL:   TTL,
		Dir:   false,
	}, nil

}

func (adapter *ZookeeperStoreAdapter) createNode(node StoreNode) error {
	root := path.Dir(node.Key)
	var err error
	exists, _, err := adapter.client.Exists(root)
	if err != nil {
		return err
	}
	if !exists {
		err = adapter.createNode(StoreNode{
			Key:   root,
			Value: []byte{},
			TTL:   0,
			Dir:   true,
		})

		if err != nil {
			return err
		}
	}

	if node.Dir {
		_, err = adapter.client.Create(node.Key, []byte{}, 0, zk.WorldACL(zk.PermAll))
	} else {
		_, err = adapter.client.Create(node.Key, adapter.encode(node.Value, node.TTL), 0, zk.WorldACL(zk.PermAll))
	}

	if err == zk.ErrNodeExists {
		err = nil
	}

	return err
}

func (adapter *ZookeeperStoreAdapter) assertNodeIsDirectory(key string) error {
	node, err := adapter.getWithTTLPolicy(key)
	if err != nil {
		return err
	}
	if !node.Dir {
		return ErrorNodeIsNotDirectory
	}

	return nil
}
