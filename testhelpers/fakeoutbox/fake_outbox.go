package fakeoutbox

import "github.com/cloudfoundry/hm9000/models"

type FakeOutbox struct {
	StartMessages []models.QueueStartMessage
	StopMessages  []models.QueueStopMessage
}

func NewFakeOutbox() *FakeOutbox {
	return &FakeOutbox{
		StartMessages: []models.QueueStartMessage{},
		StopMessages:  []models.QueueStopMessage{},
	}
}

func (outbox *FakeOutbox) Enqueue(startMessages []models.QueueStartMessage, stopMessages []models.QueueStopMessage) {
	outbox.StartMessages = append(outbox.StartMessages, startMessages...)
	outbox.StopMessages = append(outbox.StopMessages, stopMessages...)
}
