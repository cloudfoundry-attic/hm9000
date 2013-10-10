package store

import (
	"github.com/cloudfoundry/hm9000/models"
	"reflect"
)

func (store *RealStore) SaveCrashCounts(crashCounts ...models.CrashCount) error {
	return store.save(crashCounts, "/crashes", uint64(store.config.MaximumBackoffDelay().Seconds())*2)
}

func (store *RealStore) GetCrashCounts() ([]models.CrashCount, error) {
	slice, err := store.get("/crashes", reflect.TypeOf([]models.CrashCount{}), reflect.ValueOf(models.NewCrashCountFromJSON))
	return slice.Interface().([]models.CrashCount), err
}

func (store *RealStore) DeleteCrashCounts(crashCounts ...models.CrashCount) error {
	return store.delete(crashCounts, "/crashes")
}
