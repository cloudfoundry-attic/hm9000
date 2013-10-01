package hm

import (
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/desiredstatefetcher"
	"github.com/cloudfoundry/hm9000/helpers/httpclient"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"os"
	"strconv"
	"time"
)

func FetchDesiredState(l logger.Logger, conf config.Config, pollingInterval int) {
	messageBus := connectToMessageBus(l, conf)
	etcdStoreAdapter := connectToETCDStoreAdapter(l, conf)

	if pollingInterval == 0 {
		err := fetchDesiredState(l, conf, messageBus, etcdStoreAdapter)
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	} else {
		l.Info("Starting Desired State Daemon...")
		err := Daemonize(func() error {
			return fetchDesiredState(l, conf, messageBus, etcdStoreAdapter)
		}, time.Duration(pollingInterval)*time.Second, 600*time.Second, l)
		if err != nil {
			l.Error("Desired State Daemon Errored", err)
		}
		l.Info("Desired State Daemon is Down")
	}
}

func fetchDesiredState(l logger.Logger, conf config.Config, messageBus cfmessagebus.MessageBus, etcdStoreAdapter storeadapter.StoreAdapter) error {
	l.Info("Fetching Desired State")
	store := store.NewStore(conf, etcdStoreAdapter)

	fetcher := desiredstatefetcher.New(conf,
		messageBus,
		store,
		httpclient.NewHttpClient(),
		timeprovider.NewTimeProvider(),
	)

	resultChan := make(chan desiredstatefetcher.DesiredStateFetcherResult, 1)
	fetcher.Fetch(resultChan)

	result := <-resultChan
	messageBus.UnsubscribeAll()

	if result.Success {
		l.Info("Success", map[string]string{"Number of Desired Apps Fetched": strconv.Itoa(result.NumResults)})
		return nil
	} else {
		l.Error(result.Message, result.Error)
		return result.Error
	}
	return nil
}
