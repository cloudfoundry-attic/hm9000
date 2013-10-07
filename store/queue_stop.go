package store

import (
	"github.com/cloudfoundry/hm9000/models"
	"reflect"
)

func (store *RealStore) SavePendingStopMessages(messages []models.PendingStopMessage) error {
	return store.save(messages, "/stop", 0)
}

func (store *RealStore) GetPendingStopMessages() ([]models.PendingStopMessage, error) {
	slice, err := store.get("/stop", reflect.TypeOf([]models.PendingStopMessage{}), reflect.ValueOf(models.NewPendingStopMessageFromJSON))
	return slice.Interface().([]models.PendingStopMessage), err
}

func (store *RealStore) DeletePendingStopMessages(messages []models.PendingStopMessage) error {
	return store.delete(messages, "/stop")
}
