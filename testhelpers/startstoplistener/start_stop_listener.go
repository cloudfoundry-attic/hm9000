package startstoplistener

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/yagnats"
	"github.com/nats-io/nats"
)

type StartStopListener struct {
	mutex      sync.Mutex
	starts     []models.StartMessage
	stops      []models.StopMessage
	messageBus yagnats.NATSConn
}

func NewStartStopListener(messageBus yagnats.NATSConn, conf *config.Config) (*StartStopListener, string) {
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			panic(err)
		}

		stopMessage, err := models.NewStopMessageFromJSON(bodyBytes)
		if err != nil {
			panic(err)
		}
		listener.mutex.Lock()
		listener.stops = append(listener.stops, stopMessage)
		listener.mutex.Unlock()

		w.WriteHeader(200)
	}))

	return listener, server.URL
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
