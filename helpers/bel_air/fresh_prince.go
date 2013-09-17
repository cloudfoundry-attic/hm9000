package bel_air

import (
	"encoding/json"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"time"
)

type FreshPrince interface {
	Bump(key string, ttl uint64, timestamp time.Time) error
}

type RealFreshPrince struct {
	store store.Store
}

func NewFreshPrince(store store.Store) FreshPrince {
	return &RealFreshPrince{
		store: store,
	}
}

func (dj *RealFreshPrince) Bump(key string, ttl uint64, timestamp time.Time) error {
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
