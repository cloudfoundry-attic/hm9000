package startstoplistener

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

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

func NewStartStopListener(messageBus yagnats.NATSConn, conf *config.Config) (*StartStopListener, string) {
	listener := &StartStopListener{
		messageBus: messageBus,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			panic(err)
		}

		listener.mutex.Lock()
		defer listener.mutex.Unlock()

		if strings.Contains((string)(b), "is_duplicate") {
			stopMessage, err := models.NewStopMessageFromJSON(b)
			if err != nil {
				panic(err)
			}
			listener.stops = append(listener.stops, stopMessage)
		} else {
			startMessage, err := models.NewStartMessageFromJSON(b)
			if err != nil {
				panic(err)
			}
			listener.starts = append(listener.starts, startMessage)
		}

		w.WriteHeader(200)
	}))

	return listener, server.URL
}

func (listener *StartStopListener) StartCount() int {
	listener.mutex.Lock()
	defer listener.mutex.Unlock()

	cnt := len(listener.starts)
	return cnt
}

func (listener *StartStopListener) Start(i int) models.StartMessage {
	listener.mutex.Lock()
	defer listener.mutex.Unlock()

	msg := listener.starts[i]
	return msg
}

func (listener *StartStopListener) StopCount() int {
	listener.mutex.Lock()
	defer listener.mutex.Unlock()

	cnt := len(listener.stops)
	return cnt
}

func (listener *StartStopListener) Stop(i int) models.StopMessage {
	listener.mutex.Lock()
	defer listener.mutex.Unlock()

	msg := listener.stops[i]
	return msg
}

func (listener *StartStopListener) Reset() {
	listener.mutex.Lock()
	defer listener.mutex.Unlock()

	listener.starts = make([]models.StartMessage, 0)
	listener.stops = make([]models.StopMessage, 0)
}
