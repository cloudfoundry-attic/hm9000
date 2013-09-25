package store

import (
	"github.com/cloudfoundry/hm9000/models"
	"reflect"
)

func (store *RealStore) SaveQueueStartMessages(messages []models.QueueStartMessage) error {
	return store.save(messages, "/start", 0)
}

func (store *RealStore) GetQueueStartMessages() ([]models.QueueStartMessage, error) {
	slice, err := store.get("/start", reflect.TypeOf([]models.QueueStartMessage{}), reflect.ValueOf(models.NewQueueStartMessageFromJSON))
	return slice.Interface().([]models.QueueStartMessage), err
}

func (store *RealStore) DeleteQueueStartMessages(messages []models.QueueStartMessage) error {
	return store.delete(messages, "/start")
}
