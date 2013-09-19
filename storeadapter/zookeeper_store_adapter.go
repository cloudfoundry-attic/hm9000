package storeadapter

import (
	"github.com/cloudfoundry/hm9000/helpers/workerpool"
	"github.com/samuel/go-zookeeper/zk"

	"time"
)

type ZookeeperStoreAdapter struct {
	urls              []string
	client            *zk.Conn
	workerPool        *workerpool.WorkerPool
	connectionTimeout time.Duration
}

func NewZookeeperStoreAdapter(urls []string, maxConcurrentRequests int, connectionTimeout time.Duration) *ZookeeperStoreAdapter {
	return &ZookeeperStoreAdapter{
		urls:              urls,
		workerPool:        workerpool.NewWorkerPool(maxConcurrentRequests),
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
	acl := zk.WorldACL(zk.PermAll)

	for _, node := range nodes {
		node := node
		adapter.workerPool.ScheduleWork(func() {
			var err error

			exists, _, err := adapter.client.Exists(node.Key)
			if err != nil {
				results <- err
				return
			}

			if exists {
				_, err = adapter.client.Set(node.Key, node.Value, -1)
			} else {
				_, err = adapter.client.Create(node.Key, node.Value, 0, acl)
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

	// if adapter.isTimeoutError(err) {
	// 	return StoreError{reason: StoreErrorTimeout}
	// }

	return err
}

func (adapter *ZookeeperStoreAdapter) Get(key string) (StoreNode, error) {
	return StoreNode{}, nil
}

func (adapter *ZookeeperStoreAdapter) List(key string) ([]StoreNode, error) {
	return []StoreNode{}, nil
}

func (adapter *ZookeeperStoreAdapter) Delete(key string) error {
	return nil
}
