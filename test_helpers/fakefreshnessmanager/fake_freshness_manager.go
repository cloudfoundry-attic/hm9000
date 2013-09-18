package fakefreshnessmanager

import (
	"time"
)

type FakeFreshnessManager struct {
	Key       string
	TTL       uint64
	Timestamp time.Time
}

func (dj *FakeFreshnessManager) Bump(key string, ttl uint64, timestamp time.Time) error {
	dj.Key = key
	dj.TTL = ttl
	dj.Timestamp = timestamp
	return nil
}
