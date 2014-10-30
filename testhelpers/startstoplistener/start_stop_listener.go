package startstoplistener

import (
	"sync"

	"github.com/apcera/nats"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/yagnats"
)

type StartStopListener struct {
	mutex      sync.Mutex
	starts     []models.StartMessage
	stops      []models.StopMessage
	messageBus yagnats.NATSConn
}

func NewStartStopListener(messageBus yagnats.NATSConn, conf *config.Config) *StartStopListener {
	listener := &StartStopListener{
		messageBus: messageBus,
	}

	messageBus.Subscribe(conf.SenderNatsStartSubject, func(message *nats.Msg) {
		startMessage, err := models.NewStartMessageFromJSON([]byte(message.Data))
		if err != nil {
			panic(err)
		}
		listener.mutex.Lock()
		listener.starts = append(listener.starts, startMessage)
		listener.mutex.Unlock()
	})

	messageBus.Subscribe(conf.SenderNatsStopSubject, func(message *nats.Msg) {
		stopMessage, err := models.NewStopMessageFromJSON([]byte(message.Data))
		if err != nil {
			panic(err)
		}
		listener.mutex.Lock()
		listener.stops = append(listener.stops, stopMessage)
		listener.mutex.Unlock()
	})

	return listener
}

func (listener *StartStopListener) StartCount() int {
	listener.mutex.Lock()
	cnt := len(listener.starts)
	listener.mutex.Unlock()
	return cnt
}

func (listener *StartStopListener) Start(i int) models.StartMessage {
	listener.mutex.Lock()
	msg := listener.starts[i]
	listener.mutex.Unlock()
	return msg
}

func (listener *StartStopListener) StopCount() int {
	listener.mutex.Lock()
	cnt := len(listener.stops)
	listener.mutex.Unlock()
	return cnt
}

func (listener *StartStopListener) Stop(i int) models.StopMessage {
	listener.mutex.Lock()
	msg := listener.stops[i]
	listener.mutex.Unlock()
	return msg
}

func (listener *StartStopListener) Reset() {
	listener.mutex.Lock()
	listener.starts = make([]models.StartMessage, 0)
	listener.stops = make([]models.StopMessage, 0)
	listener.mutex.Unlock()
}
