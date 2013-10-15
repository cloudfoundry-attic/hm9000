package apiserver

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/store"
	"net/http"
)

type ApiServer struct {
	port         int
	store        store.Store
	timeProvider timeprovider.TimeProvider
}

func New(port int, store store.Store, timeProvider timeprovider.TimeProvider) *ApiServer {
	return &ApiServer{
		port:         port,
		store:        store,
		timeProvider: timeProvider,
	}
}

func (server *ApiServer) Start() {
	http.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		getValues := r.URL.Query()
		if len(getValues["app-guid"]) > 0 && len(getValues["app-version"]) > 0 {
			cache := storecache.New(server.store)
			err := cache.Load(server.timeProvider.Time())
			if err != nil {
				if err == cache.ActualIsNotFreshError || err == cache.DesiredIsNotFreshError {
					w.WriteHeader(http.StatusNotFound)
					return
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			appKey := cache.Key(getValues["app-guid"][0], getValues["app-version"][0])

			app, present := cache.Apps[appKey]
			if present {
				w.WriteHeader(http.StatusOK)
				w.Write(app.ToJSON())
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return
		} else {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	})
	err := http.ListenAndServe(fmt.Sprintf(":%d", server.port), nil)
	panic(err)
}
