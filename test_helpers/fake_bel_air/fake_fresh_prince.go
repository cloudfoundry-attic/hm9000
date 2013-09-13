package fake_bel_air

import (
	"time"
)

type FakeFreshPrince struct {
	Key       string
	TTL       uint64
	Timestamp time.Time
}

func (dj *FakeFreshPrince) Bump(key string, ttl uint64, timestamp time.Time) error {
	dj.Key = key
	dj.TTL = ttl
	dj.Timestamp = timestamp
	return nil
}
