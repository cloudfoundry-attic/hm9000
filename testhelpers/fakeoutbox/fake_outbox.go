package fakeoutbox

import "github.com/cloudfoundry/hm9000/models"

type FakeOutbox struct {
	QueuedStartMessages map[string]models.QueueStartMessage
	QueuedStopMessages  map[string]models.QueueStopMessage
	Error               error
}

func NewFakeOutbox() *FakeOutbox {
	return &FakeOutbox{
		QueuedStartMessages: map[string]models.QueueStartMessage{},
		QueuedStopMessages:  map[string]models.QueueStopMessage{},
	}
}

func (outbox *FakeOutbox) Enqueue(startMessages []models.QueueStartMessage, stopMessages []models.QueueStopMessage) error {
	for _, message := range startMessages {
		outbox.QueuedStartMessages[message.StoreKey()] = message
	}
	for _, message := range stopMessages {
		outbox.QueuedStopMessages[message.StoreKey()] = message
	}
	return outbox.Error
}
