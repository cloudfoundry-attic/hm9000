package store

import (
	"github.com/coreos/go-etcd/etcd"
)

type ETCDStore struct {
	url    string
	client *etcd.Client
}

func NewETCDStore(url string) *ETCDStore {
	return &ETCDStore{
		url: url,
	}
}

func (store *ETCDStore) Connect() error {
	store.client = etcd.NewClient()
	store.client.SetCluster([]string{store.url})

	return nil
}

func (store *ETCDStore) isTimeoutError(err error) bool {
	return err != nil && err.Error() == "Cannot reach servers"
}

func (store *ETCDStore) Set(key string, value []byte, ttl uint64) error {
	_, err := store.client.Set(key, string(value), ttl)
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
