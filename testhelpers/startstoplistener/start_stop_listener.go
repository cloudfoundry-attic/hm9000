package startstoplistener

import (
	"encoding/json"
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/models"
)

type StartStopListener struct {
	Starts     []models.StartMessage
	Stops      []models.StopMessage
	messageBus cfmessagebus.MessageBus
}

func NewStartStopListener(messageBus cfmessagebus.MessageBus) *StartStopListener {
	listener := &StartStopListener{
		messageBus: messageBus,
	}

	messageBus.Subscribe("health.start", func(payload []byte) {
		startMessage := models.StartMessage{}
		json.Unmarshal(payload, &startMessage)
		listener.Starts = append(listener.Starts, startMessage)
	})

	messageBus.Subscribe("health.stop", func(payload []byte) {
		stopMessage := models.StopMessage{}
		json.Unmarshal(payload, &stopMessage)
		listener.Stops = append(listener.Stops, stopMessage)
	})

	return listener
}

func (listener *StartStopListener) Reset() {
	listener.Starts = make([]models.StartMessage, 0)
	listener.Stops = make([]models.StopMessage, 0)
}
