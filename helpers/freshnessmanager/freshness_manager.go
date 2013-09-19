package freshnessmanager

import (
	"encoding/json"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"time"
)

type FreshnessManager interface {
	Bump(key string, ttl uint64, timestamp time.Time) error
}

type RealFreshnessManager struct {
	storeAdapter storeadapter.StoreAdapter
}

func NewFreshnessManager(storeAdapter storeadapter.StoreAdapter) FreshnessManager {
	return &RealFreshnessManager{
		storeAdapter: storeAdapter,
	}
}

func (dj *RealFreshnessManager) Bump(key string, ttl uint64, timestamp time.Time) error {
	var jsonTimestamp []byte
	oldTimestamp, err := dj.storeAdapter.Get(key)

	if err == nil {
		jsonTimestamp = oldTimestamp.Value
	} else {
		jsonTimestamp, _ = json.Marshal(models.FreshnessTimestamp{Timestamp: timestamp.Unix()})
	}

	return dj.storeAdapter.Set([]storeadapter.StoreNode{
		storeadapter.StoreNode{
			Key:   key,
			Value: jsonTimestamp,
			TTL:   ttl,
		},
	})
}
