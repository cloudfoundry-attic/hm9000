package fakeoutbox

import "github.com/cloudfoundry/hm9000/models"

type FakeOutbox struct {
	PendingStartMessages map[string]models.PendingStartMessage
	PendingStopMessages  map[string]models.PendingStopMessage
	Error               error
}

func NewFakeOutbox() *FakeOutbox {
	return &FakeOutbox{
		PendingStartMessages: map[string]models.PendingStartMessage{},
		PendingStopMessages:  map[string]models.PendingStopMessage{},
	}
}

func (outbox *FakeOutbox) Enqueue(startMessages []models.PendingStartMessage, stopMessages []models.PendingStopMessage) error {
	for _, message := range startMessages {
		outbox.PendingStartMessages[message.StoreKey()] = message
	}
	for _, message := range stopMessages {
		outbox.PendingStopMessages[message.StoreKey()] = message
	}
	return outbox.Error
}
