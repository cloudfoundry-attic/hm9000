package store

import (
	"encoding/json"
	"time"

	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/storeadapter"
)

func (store *RealStore) BumpActualFreshness(timestamp time.Time) error {
	return store.bumpFreshness(store.SchemaRoot()+store.config.ActualFreshnessKey, store.config.ActualFreshnessTTL(), timestamp)
}

func (store *RealStore) RevokeActualFreshness() error {
	return store.adapter.Delete(store.SchemaRoot() + store.config.ActualFreshnessKey)
}

func (store *RealStore) bumpFreshness(key string, ttl uint64, timestamp time.Time) error {
	var jsonTimestamp []byte
	oldTimestamp, err := store.adapter.Get(key)

	if err == nil {
		jsonTimestamp = oldTimestamp.Value
	} else {
		jsonTimestamp, _ = json.Marshal(models.FreshnessTimestamp{Timestamp: timestamp.Unix()})
	}

	return store.adapter.SetMulti([]storeadapter.StoreNode{
		{
			Key:   key,
			Value: jsonTimestamp,
			TTL:   ttl,
		},
	})
}

func (store *RealStore) IsActualStateFresh(currentTime time.Time) (bool, error) {
	node, err := store.adapter.Get(store.SchemaRoot() + store.config.ActualFreshnessKey)
	if err != nil && err.(storeadapter.Error).Type() == storeadapter.ErrorKeyNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	freshnessTimestamp := models.FreshnessTimestamp{}
	err = json.Unmarshal(node.Value, &freshnessTimestamp)
	if err != nil {
		return false, err
	}

	isUpToDate := currentTime.Sub(time.Unix(freshnessTimestamp.Timestamp, 0)) >= time.Duration(store.config.ActualFreshnessTTL())*time.Second
	return isUpToDate, nil
}

func (store *RealStore) VerifyFreshness(time time.Time) error {
	actualFresh, err := store.IsActualStateFresh(time)
	if err != nil {
		return err
	}

	if !actualFresh {
		return ActualIsNotFreshError
	}

	return nil
}
