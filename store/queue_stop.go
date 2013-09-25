package store

import (
	"github.com/cloudfoundry/hm9000/models"
	"reflect"
)

func (store *RealStore) SaveQueueStopMessages(messages []models.QueueStopMessage) error {
	return store.save(messages, "/stop", 0)
}

func (store *RealStore) GetQueueStopMessages() ([]models.QueueStopMessage, error) {
	slice, err := store.get("/stop", reflect.TypeOf([]models.QueueStopMessage{}), reflect.ValueOf(models.NewQueueStopMessageFromJSON))
	return slice.Interface().([]models.QueueStopMessage), err
}

func (store *RealStore) DeleteQueueStopMessages(messages []models.QueueStopMessage) error {
	return store.delete(messages, "/stop")
}
