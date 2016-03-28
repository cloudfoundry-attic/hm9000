package actualstatelisteners

import (
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

type heartbeatListener struct {
	logger lager.Logger
	config *config.Config
	syncer Syncer
}

func NewHttpListener(logger lager.Logger, config *config.Config, syncer Syncer) (http.Handler, error) {
	listeners := map[string]http.Handler{
		"dea_heartbeat_listener": NewHeartbeatListener(logger, config, syncer),
	}

	routes := rata.Routes{
		{Method: "POST", Name: "dea_heartbeat_listener", Path: "/dea/heartbeat"},
	}

	return rata.NewRouter(routes, listeners)
}

func NewHeartbeatListener(logger lager.Logger,
	config *config.Config,
	syncer Syncer) *heartbeatListener {

	return &heartbeatListener{
		logger: logger,
		config: config,
		syncer: syncer,
	}
}

func (listener *heartbeatListener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	listener.logger.Debug("Got an HTTP heartbeat")

	bodyBytes, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil || len(bodyBytes) == 0 {
		listener.logger.Error("Failed to read dea heartbeat body", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	heartbeat, e := models.NewHeartbeatFromJSON(bodyBytes)
	if e != nil {
		listener.logger.Error("Failed to unmarshal dea heartbeat", e)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	listener.logger.Debug("Decoded the HTTP heartbeat")
	listener.syncer.Heartbeat(heartbeat)
	w.WriteHeader(http.StatusAccepted)
}
