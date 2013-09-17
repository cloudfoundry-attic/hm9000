package store

import (
	"github.com/cloudfoundry/hm9000/helpers/worker_pool"
	"github.com/coreos/go-etcd/etcd"
)

type ETCDStore struct {
	urls       []string
	client     *etcd.Client
	workerPool *worker_pool.WorkerPool
}

func NewETCDStore(urls []string, maxConcurrentRequests int) *ETCDStore {
	return &ETCDStore{
		urls:       urls,
		workerPool: worker_pool.NewWorkerPool(maxConcurrentRequests),
	}
}

func (store *ETCDStore) Connect() error {
	store.client = etcd.NewClient()
	store.client.SetCluster(store.urls)

	return nil
}

func (store *ETCDStore) Disconnect() error {
	store.workerPool.StopWorkers()

	return nil
}

func (store *ETCDStore) isTimeoutError(err error) bool {
	return err != nil && err.Error() == "Cannot reach servers"
}

func (store *ETCDStore) Set(nodes []StoreNode) error {
	results := make(chan error, len(nodes))

	for _, node := range nodes {
		node := node
		store.workerPool.ScheduleWork(func() {
			_, err := store.client.Set(node.Key, string(node.Value), node.TTL)
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

	if store.isTimeoutError(err) {
		return ETCDError{reason: ETCDErrorTimeout}
	}

	return err
}

func (store *ETCDStore) Get(key string) (StoreNode, error) {
	responses, err := store.client.Get(key)
	if store.isTimeoutError(err) {
		return StoreNode{}, ETCDError{reason: ETCDErrorTimeout}
	}

	if len(responses) == 0 {
		return StoreNode{}, ETCDError{reason: ETCDErrorKeyNotFound}
	}
	if err != nil {
		return StoreNode{}, err
	}
	if len(responses) > 1 || responses[0].Key != key {
		return StoreNode{}, ETCDError{reason: ETCDErrorIsDirectory}
	}

	return StoreNode{
		Key:   responses[0].Key,
		Value: []byte(responses[0].Value),
		Dir:   responses[0].Dir,
		TTL:   uint64(responses[0].TTL),
	}, nil
}

func (store *ETCDStore) List(key string) ([]StoreNode, error) {
	responses, err := store.client.Get(key)
	if store.isTimeoutError(err) {
		return []StoreNode{}, ETCDError{reason: ETCDErrorTimeout}
	}

	if err != nil {
		return []StoreNode{}, err
	}

	if len(responses) == 0 {
		return []StoreNode{}, nil
	}

	if responses[0].Key == key {
		return []StoreNode{}, ETCDError{reason: ETCDErrorIsNotDirectory}
	}

	values := make([]StoreNode, len(responses))

	for i, response := range responses {
		values[i] = StoreNode{
			Key:   response.Key,
			Value: []byte(response.Value),
			Dir:   response.Dir,
			TTL:   uint64(response.TTL),
		}
	}

	return values, nil
}

func (store *ETCDStore) Delete(key string) error {
	_, err := store.client.Delete(key)
	if store.isTimeoutError(err) {
		return ETCDError{reason: ETCDErrorTimeout}
	}

	return err
}
