package handlers

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/hm9000/apiserver"
	"github.com/cloudfoundry/hm9000/store"
	"code.cloudfoundry.org/clock"
	"github.com/tedsuo/rata"
)

func New(logger lager.Logger, store store.Store, clock clock.Clock) (http.Handler, error) {
	handlers := map[string]http.Handler{
		"bulk_app_state": NewBulkAppStateHandler(logger, store, clock),
	}

	return rata.NewRouter(apiserver.Routes, handlers)
}
