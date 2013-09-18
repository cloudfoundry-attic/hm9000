package freshnessmanager

import (
	"encoding/json"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"time"
)

type FreshnessManager interface {
	Bump(key string, ttl uint64, timestamp time.Time) error
}

type RealFreshnessManager struct {
	store store.Store
}

func NewFreshnessManager(store store.Store) FreshnessManager {
	return &RealFreshnessManager{
		store: store,
	}
}

func (dj *RealFreshnessManager) Bump(key string, ttl uint64, timestamp time.Time) error {
	var jsonTimestamp []byte
	oldTimestamp, err := dj.store.Get(key)

	if err == nil {
		jsonTimestamp = oldTimestamp.Value
	} else {
		jsonTimestamp, _ = json.Marshal(models.FreshnessTimestamp{Timestamp: timestamp.Unix()})
	}

	return dj.store.Set([]store.StoreNode{
		store.StoreNode{
			Key:   key,
			Value: jsonTimestamp,
			TTL:   ttl,
		},
	})
}
