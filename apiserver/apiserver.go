package apiserver

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"net/http"
	"strconv"
)

type ApiServer struct {
	port         int
	store        store.Store
	timeProvider timeprovider.TimeProvider
	conf         config.Config
	logger       logger.Logger
}

func New(port int, store store.Store, timeProvider timeprovider.TimeProvider, conf config.Config, logger logger.Logger) *ApiServer {
	return &ApiServer{
		port:         port,
		store:        store,
		timeProvider: timeProvider,
		conf:         conf,
		logger:       logger,
	}
}

func (server *ApiServer) Start() {
	http.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		var responseCode int
		var responseBody []byte

		defer func() {
			w.WriteHeader(responseCode)
			w.Write(responseBody)
			server.logger.Info("Handling API Request", map[string]string{
				"URL":          r.URL.String(),
				"ResponseCode": strconv.Itoa(responseCode),
			})
		}()

		auth, authIsPresent := r.Header["Authorization"]

		if !authIsPresent {
			responseCode = http.StatusUnauthorized
			return
		}

		basicAuth, err := models.DecodeBasicAuthInfo(auth[0])

		if !(err == nil && basicAuth.User == server.conf.APIServerUser && basicAuth.Password == server.conf.APIServerPassword) {
			responseCode = http.StatusUnauthorized
			return
		}

		getValues := r.URL.Query()
		if len(getValues["app-guid"]) > 0 && len(getValues["app-version"]) > 0 {
			cache := storecache.New(server.store)
			err := cache.Load(server.timeProvider.Time())
			if err != nil {
				if err == cache.ActualIsNotFreshError || err == cache.DesiredIsNotFreshError {
					responseCode = http.StatusNotFound
					return
				} else {
					responseCode = http.StatusInternalServerError
					return
				}
			}

			appKey := cache.Key(getValues["app-guid"][0], getValues["app-version"][0])

			app, present := cache.Apps[appKey]
			if present {
				responseCode = http.StatusOK
				responseBody = app.ToJSON()
				return
			} else {
				responseCode = http.StatusNotFound
				return
			}
		} else {
			responseCode = http.StatusBadRequest
			return
		}
	})
	err := http.ListenAndServe(fmt.Sprintf(":%d", server.port), nil)
	panic(err)
}
