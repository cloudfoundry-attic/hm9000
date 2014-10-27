package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/store"
)

type bulkHandler struct {
	logger       logger.Logger
	store        store.Store
	timeProvider timeprovider.TimeProvider
}

type AppStateRequest struct {
	AppGuid    string `json:"droplet"`
	AppVersion string `json:"version"`
}

func NewBulkAppStateHandler(logger logger.Logger, store store.Store, timeProvider timeprovider.TimeProvider) http.Handler {
	return &bulkHandler{
		logger:       logger,
		store:        store,
		timeProvider: timeProvider,
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
		handler.logger.Error("Failed to handle bulk_app_state request", err, map[string]string{
			"payload":      string(bodyBytes),
			"elapsed time": fmt.Sprintf("%s", time.Since(startTime)),
		})
		w.Write([]byte("{}"))
		return
	}

	err = handler.store.VerifyFreshness(handler.timeProvider.Time())
	if err != nil {
		handler.logger.Error("Failed to handle bulk_app_state request", err, map[string]string{
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
		handler.logger.Error("Failed to handle bulk_app_state request", err, map[string]string{
			"payload":      string(bodyBytes),
			"elapsed time": fmt.Sprintf("%s", time.Since(startTime)),
		})
	}

	w.Write([]byte(appsJson))
}
