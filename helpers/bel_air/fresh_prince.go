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
	var jsonTimestamp string
	oldTimestamp, err := dj.store.Get(key)

	if err == nil {
		jsonTimestamp = oldTimestamp.Value
	} else {
		jsonBytes, _ := json.Marshal(models.FreshnessTimestamp{Timestamp: timestamp.Unix()})
		jsonTimestamp = string(jsonBytes)
	}

	return dj.store.Set(key, jsonTimestamp, ttl)
}
