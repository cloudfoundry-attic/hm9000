package apiserver

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/apcera/nats"
	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/yagnats"
)

type ApiServer struct {
	messageBus   yagnats.ApceraWrapperNATSClient
	store        store.Store
	timeProvider timeprovider.TimeProvider
	logger       logger.Logger
}

type AppStateRequest struct {
	AppGuid    string `json:"droplet"`
	AppVersion string `json:"version"`
}

func New(messageBus yagnats.ApceraWrapperNATSClient, store store.Store, timeProvider timeprovider.TimeProvider, logger logger.Logger) *ApiServer {
	return &ApiServer{
		messageBus:   messageBus,
		store:        store,
		timeProvider: timeProvider,
		logger:       logger,
	}
}

func (server *ApiServer) Listen() {
	server.handle("app.state", server.handleAppStateRequest)
	server.handle("app.state.bulk", server.handleBulkAppStateRequest)
}

func (server *ApiServer) handle(topic string, handler func(message *nats.Msg) ([]byte, error)) {
	server.messageBus.SubscribeWithQueue(topic, "hm9000", func(message *nats.Msg) {
		if message.Reply == "" {
			return
		}

		t := time.Now()

		var err error
		var response []byte = []byte("{}")

		err = server.store.VerifyFreshness(server.timeProvider.Time())
		if err == nil {
			response, err = handler(message)
		}

		if err != nil {
			server.messageBus.Publish(message.Reply, []byte("{}"))
			server.logger.Error(fmt.Sprintf("Failed to handle %s request", topic), err, map[string]string{
				"payload":      string(message.Data),
				"elapsed time": fmt.Sprintf("%s", time.Since(t)),
			})
			return
		} else {
			server.messageBus.Publish(message.Reply, response)
			server.logger.Info(fmt.Sprintf("Responded succesfully to %s request", topic), map[string]string{
				"payload":      string(message.Data),
				"elapsed time": fmt.Sprintf("%s", time.Since(t)),
			})
		}
	})
}

func (server *ApiServer) handleAppStateRequest(message *nats.Msg) ([]byte, error) {
	var request AppStateRequest
	err := json.Unmarshal([]byte(message.Data), &request)
	if err != nil {
		return nil, err
	}

	app, err := server.store.GetApp(request.AppGuid, request.AppVersion)
	if err != nil {
		return nil, err
	}

	response := app.ToJSON()
	return response, err
}

func (server *ApiServer) handleBulkAppStateRequest(message *nats.Msg) ([]byte, error) {
	requests := make([]AppStateRequest, 0)
	err := json.Unmarshal([]byte(message.Data), &requests)
	if err != nil {
		return nil, err
	}

	var apps = make(map[string]interface{})
	for _, request := range requests {
		app, err := server.store.GetApp(request.AppGuid, request.AppVersion)
		if err == nil {
			apps[app.AppGuid] = app
		}
	}

	return json.Marshal(apps)
}
