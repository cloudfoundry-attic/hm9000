package handlers

import (
	"net/http"

	"github.com/cloudfoundry/hm9000/apiserver"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/pivotal-golang/clock"
	"github.com/tedsuo/rata"
)

func New(logger logger.Logger, store store.Store, clock clock.Clock) (http.Handler, error) {
	handlers := map[string]http.Handler{
		"bulk_app_state": NewBulkAppStateHandler(logger, store, clock),
	}

	return rata.NewRouter(apiserver.Routes, handlers)
}
