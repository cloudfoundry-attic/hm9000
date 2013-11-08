package store

import (
	"github.com/cloudfoundry/hm9000/storeadapter"
	"strconv"
)

func (store *RealStore) SaveMetric(metric string, value int) error {
	node := storeadapter.StoreNode{
		Key:   "/metrics/" + metric,
		Value: []byte(strconv.Itoa(value)),
	}
	return store.adapter.Set([]storeadapter.StoreNode{node})
}

func (store *RealStore) GetMetric(metric string) (int, error) {
	node, err := store.adapter.Get("/metrics/" + metric)
	if err != nil {
		return -1, err
	}

	return strconv.Atoi(string(node.Value))
}
