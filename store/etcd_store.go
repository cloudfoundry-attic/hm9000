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

func (store *ETCDStore) Set(key string, value string, ttl uint64) error {
	_, err := store.client.Set(key, value, ttl)
	return err
}

func (store *ETCDStore) Get(key string) (string, error) {
	responses, err := store.client.Get(key)
	if len(responses) == 0 {
		return "", ETCDError{reason: ETCDErrorKeyNotFound}
	}
	if len(responses) > 1 || responses[0].Key != key {
		return "", ETCDError{reason: ETCDErrorIsDirectory}
	}
	return responses[0].Value, err
}

func (store *ETCDStore) List(key string) ([]StoreNode, error) {
	responses, err := store.client.Get(key)
	if err != nil {
		return []StoreNode{}, err
	}

	if responses[0].Key == key {
		return []StoreNode{}, ETCDError{reason: ETCDErrorIsNotDirectory}
	}

	values := make([]StoreNode, len(responses))

	for i, response := range responses {
		values[i] = StoreNode{
			Key:   response.Key,
			Value: response.Value,
			Dir:   response.Dir,
		}
	}

	return values, nil
}

func (store *ETCDStore) Delete(key string) error {
	_, err := store.client.Delete(key)
	return err
}
