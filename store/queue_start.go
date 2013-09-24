package store

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
)

func (store *RealStore) SaveQueueStartMessages(messages []models.QueueStartMessage) error {
	nodes := make([]storeadapter.StoreNode, len(messages))
	for i, message := range messages {
		nodes[i] = storeadapter.StoreNode{
			Key:   "/start/" + message.StoreKey(),
			Value: message.ToJSON(),
			TTL:   0,
		}
	}

	return store.adapter.Set(nodes)
}

func (store *RealStore) GetQueueStartMessages() ([]models.QueueStartMessage, error) {
	nodes, err := store.fetchNodesUnderDir("/start")
	if err != nil {
		return []models.QueueStartMessage{}, err
	}

	messages := make([]models.QueueStartMessage, len(nodes))
	for i, node := range nodes {
		messages[i], err = models.NewQueueStartMessageFromJSON(node.Value)
		if err != nil {
			return []models.QueueStartMessage{}, err
		}
	}

	return messages, nil
}

func (store *RealStore) DeleteQueueStartMessages(messages []models.QueueStartMessage) error {
	for _, message := range messages {
		err := store.adapter.Delete("/start/" + message.StoreKey())
		if err != nil {
			return err
		}
	}
	return nil
}
