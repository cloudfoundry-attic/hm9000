package handlers

import (
	"net/http"

	"github.com/cloudfoundry/hm9000/apiserver"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/rata"
)

func New(logger lager.Logger, store store.Store, clock clock.Clock) (http.Handler, error) {
	handlers := map[string]http.Handler{
		"bulk_app_state": NewBulkAppStateHandler(logger, store, clock),
	}

	return rata.NewRouter(apiserver.Routes, handlers)
}
