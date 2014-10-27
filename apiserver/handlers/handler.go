package handlers

import (
	"net/http"

	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/cloudfoundry/hm9000/apiserver"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/tedsuo/rata"
)

func New(logger logger.Logger, store store.Store, timeProvider timeprovider.TimeProvider) (http.Handler, error) {
	handlers := map[string]http.Handler{
		"bulk_app_state": NewBulkAppStateHandler(logger, store, timeProvider),
	}

	return rata.NewRouter(apiserver.Routes, handlers)
}
