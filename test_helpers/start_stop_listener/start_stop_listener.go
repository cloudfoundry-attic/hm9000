package start_stop_listener

import (
	"encoding/json"
	"github.com/cloudfoundry/go_cfmessagebus"
	. "github.com/cloudfoundry/hm9000/models"
)

type StartStopListener struct {
	Starts     []StartMessage
	Stops      []StopMessage
	messageBus cfmessagebus.MessageBus
}

func NewStartStopListener(messageBus cfmessagebus.MessageBus) *StartStopListener {
	listener := &StartStopListener{
		messageBus: messageBus,
	}

	messageBus.Subscribe("health.start", func(payload []byte) {
		startMessage := StartMessage{}
		json.Unmarshal(payload, &startMessage)

		listener.Starts = append(listener.Starts, startMessage)
	})

	messageBus.Subscribe("health.stop", func(payload []byte) {
		stopMessage := StopMessage{}
		json.Unmarshal(payload, &stopMessage)
		listener.Stops = append(listener.Stops, stopMessage)
	})

	return listener
}

func (listener *StartStopListener) Reset() {
	listener.Starts = make([]StartMessage, 0)
	listener.Stops = make([]StopMessage, 0)
}
