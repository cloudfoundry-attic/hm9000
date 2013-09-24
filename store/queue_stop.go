package store

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
)

func (store *RealStore) SaveQueueStopMessages(messages []models.QueueStopMessage) error {
	nodes := make([]storeadapter.StoreNode, len(messages))
	for i, message := range messages {
		nodes[i] = storeadapter.StoreNode{
			Key:   "/stop/" + message.StoreKey(),
			Value: message.ToJSON(),
			TTL:   0,
		}
	}

	return store.adapter.Set(nodes)
}

func (store *RealStore) GetQueueStopMessages() ([]models.QueueStopMessage, error) {
	nodes, err := store.fetchNodesUnderDir("/stop")
	if err != nil {
		return []models.QueueStopMessage{}, err
	}

	messages := make([]models.QueueStopMessage, len(nodes))
	for i, node := range nodes {
		messages[i], err = models.NewQueueStopMessageFromJSON(node.Value)
		if err != nil {
			return []models.QueueStopMessage{}, err
		}
	}

	return messages, nil
}

func (store *RealStore) DeleteQueueStopMessages(messages []models.QueueStopMessage) error {
	for _, message := range messages {
		err := store.adapter.Delete("/stop/" + message.StoreKey())
		if err != nil {
			return err
		}
	}
	return nil
}
