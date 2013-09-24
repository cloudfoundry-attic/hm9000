package outbox

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/models"
)

type Outbox interface {
	Enqueue([]models.QueueStartMessage, []models.QueueStopMessage)
}

type LoggingOutbox struct {
	logger logger.Logger
}

func NewLoggingOutbox(logger logger.Logger) *LoggingOutbox {
	return &LoggingOutbox{
		logger: logger,
	}
}

func (outbox *LoggingOutbox) Enqueue(startMessages []models.QueueStartMessage, stopMessages []models.QueueStopMessage) {
	for _, message := range startMessages {
		outbox.logger.Info("Enqueing Start Message", map[string]string{
			"AppGuid":        message.AppGuid,
			"AppVersion":     message.AppVersion,
			"IndicesToStart": fmt.Sprintf("%v", message.IndicesToStart),
		})
	}

	for _, message := range stopMessages {
		outbox.logger.Info("Enqueing Stop Message", map[string]string{
			"InstanceGuid": message.InstanceGuid,
		})
	}
}
