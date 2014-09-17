package startstoplistener

import (
	"github.com/apcera/nats"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/yagnats"
)

type StartStopListener struct {
	Starts     []models.StartMessage
	Stops      []models.StopMessage
	messageBus yagnats.ApceraWrapperNATSClient
}

func NewStartStopListener(messageBus yagnats.ApceraWrapperNATSClient, conf *config.Config) *StartStopListener {
	listener := &StartStopListener{
		messageBus: messageBus,
	}

	messageBus.Subscribe(conf.SenderNatsStartSubject, func(message *nats.Msg) {
		startMessage, err := models.NewStartMessageFromJSON([]byte(message.Data))
		if err != nil {
			panic(err)
		}
		listener.Starts = append(listener.Starts, startMessage)
	})

	messageBus.Subscribe(conf.SenderNatsStopSubject, func(message *nats.Msg) {
		stopMessage, err := models.NewStopMessageFromJSON([]byte(message.Data))
		if err != nil {
			panic(err)
		}
		listener.Stops = append(listener.Stops, stopMessage)
	})

	return listener
}

func (listener *StartStopListener) Reset() {
	listener.Starts = make([]models.StartMessage, 0)
	listener.Stops = make([]models.StopMessage, 0)
}
