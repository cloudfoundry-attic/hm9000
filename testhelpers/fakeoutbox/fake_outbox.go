package fakeoutbox

import "github.com/cloudfoundry/hm9000/models"

type FakeOutbox struct {
	StartMessages map[string]models.QueueStartMessage
	StopMessages  map[string]models.QueueStopMessage
	Error         error
}

func NewFakeOutbox() *FakeOutbox {
	return &FakeOutbox{
		StartMessages: map[string]models.QueueStartMessage{},
		StopMessages:  map[string]models.QueueStopMessage{},
	}
}

func (outbox *FakeOutbox) Enqueue(startMessages []models.QueueStartMessage, stopMessages []models.QueueStopMessage) error {
	for _, message := range startMessages {
		outbox.StartMessages[message.StoreKey()] = message
	}
	for _, message := range stopMessages {
		outbox.StopMessages[message.StoreKey()] = message
	}
	return outbox.Error
}
