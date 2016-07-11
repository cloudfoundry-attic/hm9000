package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/hm9000/store"
	"code.cloudfoundry.org/clock"
)

type bulkHandler struct {
	logger lager.Logger
	store  store.Store
	clock  clock.Clock
}

type AppStateRequest struct {
	AppGuid    string `json:"droplet"`
	AppVersion string `json:"version"`
}

func NewBulkAppStateHandler(logger lager.Logger, store store.Store, clock clock.Clock) http.Handler {
	return &bulkHandler{
		logger: logger,
		store:  store,
		clock:  clock,
	}
}

func (handler *bulkHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	requests := make([]AppStateRequest, 0)

	bodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handler.logger.Error("Failed to handle bulk_app_state request", err)
		w.Write([]byte("{}"))
	}

	err = json.Unmarshal(bodyBytes, &requests)
	if err != nil {
		handler.logger.Error("Failed to handle bulk_app_state request", err, lager.Data{
			"payload":      string(bodyBytes),
			"elapsed time": fmt.Sprintf("%s", time.Since(startTime)),
		})
		w.Write([]byte("{}"))
		return
	}

	err = handler.store.VerifyFreshness(handler.clock.Now())
	if err != nil {
		handler.logger.Error("Failed to handle bulk_app_state request", err, lager.Data{
			"payload":      string(bodyBytes),
			"elapsed time": fmt.Sprintf("%s", time.Since(startTime)),
		})
		w.Write([]byte("{}"))
		return
	}

	var apps = make(map[string]interface{})
	for _, request := range requests {
		app, err := handler.store.GetApp(request.AppGuid, request.AppVersion)
		if err == nil {
			apps[app.AppGuid] = app
		}
	}

	appsJson, err := json.Marshal(apps)
	if err != nil {
		handler.logger.Error("Failed to handle bulk_app_state request", err, lager.Data{
			"payload":      string(bodyBytes),
			"elapsed time": fmt.Sprintf("%s", time.Since(startTime)),
		})
	}

	w.Write([]byte(appsJson))
}
