package apiserver

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
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

		queryParams := r.URL.Query()
		if len(queryParams["app-guid"]) > 0 && len(queryParams["app-version"]) > 0 {
			err := server.store.VerifyFreshness(server.timeProvider.Time())
			if err == store.ActualAndDesiredAreNotFreshError || err == store.ActualIsNotFreshError || err == store.DesiredIsNotFreshError {
				responseCode = http.StatusNotFound
				return
			} else if err != nil {
				responseCode = http.StatusInternalServerError
				return
			}

			app, err := server.store.GetApp(queryParams["app-guid"][0], queryParams["app-version"][0])

			if err == store.AppNotFoundError {
				responseCode = http.StatusNotFound
				return
			} else if err != nil {
				responseCode = http.StatusInternalServerError
				return
			} else {
				responseCode = http.StatusOK
				responseBody = app.ToJSON()
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
