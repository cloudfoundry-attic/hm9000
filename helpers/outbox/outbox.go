package outbox

import (
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
)

type Outbox interface {
	Enqueue([]models.QueueStartMessage, []models.QueueStopMessage) error
}

type RealOutbox struct {
	store                 store.Store
	logger                logger.Logger
	existingStartMessages map[string]bool
	existingStopMessages  map[string]bool
}

func New(store store.Store, logger logger.Logger) Outbox {
	return &RealOutbox{
		store:                 store,
		logger:                logger,
		existingStartMessages: map[string]bool{},
		existingStopMessages:  map[string]bool{},
	}
}

func (outbox *RealOutbox) Enqueue(startMessages []models.QueueStartMessage, stopMessages []models.QueueStopMessage) error {
	err := outbox.fetchCurrentlyEnqueuedMessages()
	if err != nil {
		return err
	}

	dedupedStartMessages := []models.QueueStartMessage{}
	dedupedStopMessages := []models.QueueStopMessage{}

	for _, message := range startMessages {
		if outbox.existingStartMessages[message.StoreKey()] {
			outbox.logger.Info("Skipping Already Enqueued Start Message", message.LogDescription())
		} else {
			outbox.logger.Info("Enqueuing Start Message", message.LogDescription())
			dedupedStartMessages = append(dedupedStartMessages, message)
		}
	}

	for _, message := range stopMessages {
		if outbox.existingStopMessages[message.StoreKey()] {
			outbox.logger.Info("Skipping Already Enqueued Stop Message", message.LogDescription())
		} else {
			outbox.logger.Info("Enqueuing Stop Message", message.LogDescription())
			dedupedStopMessages = append(dedupedStopMessages, message)
		}
	}

	err = outbox.store.SaveQueueStartMessages(dedupedStartMessages)
	if err != nil {
		return err
	}

	err = outbox.store.SaveQueueStopMessages(dedupedStopMessages)
	if err != nil {
		return err
	}
	return nil
}

func (outbox *RealOutbox) fetchCurrentlyEnqueuedMessages() error {
	starts, err := outbox.store.GetQueueStartMessages()
	if err != nil {
		return err
	}
	stops, err := outbox.store.GetQueueStopMessages()
	if err != nil {
		return err
	}
	for _, start := range starts {
		outbox.existingStartMessages[start.StoreKey()] = true
	}
	for _, stop := range stops {
		outbox.existingStopMessages[stop.StoreKey()] = true
	}
	return nil
}
