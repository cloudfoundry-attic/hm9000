package store

import (
	"encoding/json"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"time"
)

func (store *RealStore) BumpDesiredFreshness(timestamp time.Time) error {
	return store.bumpFreshness(store.config.DesiredFreshnessKey, store.config.DesiredFreshnessTTL, timestamp)
}

func (store *RealStore) BumpActualFreshness(timestamp time.Time) error {
	return store.bumpFreshness(store.config.ActualFreshnessKey, store.config.ActualFreshnessTTL, timestamp)
}

func (store *RealStore) bumpFreshness(key string, ttl uint64, timestamp time.Time) error {
	var jsonTimestamp []byte
	oldTimestamp, err := store.adapter.Get(key)

	if err == nil {
		jsonTimestamp = oldTimestamp.Value
	} else {
		jsonTimestamp, _ = json.Marshal(models.FreshnessTimestamp{Timestamp: timestamp.Unix()})
	}

	return store.adapter.Set([]storeadapter.StoreNode{
		storeadapter.StoreNode{
			Key:   key,
			Value: jsonTimestamp,
			TTL:   ttl,
		},
	})
}
