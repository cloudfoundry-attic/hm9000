package hm

import (
	"os"
	"strconv"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/desiredstatefetcher"
	"github.com/cloudfoundry/hm9000/helpers/httpclient"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

func FetchDesiredState(l lager.Logger, conf *config.Config, poll bool) {
	store := connectToStore(l, conf)
	clock := buildClock(l)
	client := httpclient.NewHttpClient(conf.SkipSSLVerification, conf.FetcherNetworkTimeout())

	if poll {
		l.Info("Starting Desired State Fetcher...")

		f := &Component{
			component:       "fetcher",
			conf:            conf,
			pollingInterval: conf.FetcherPollingInterval(),
			timeout:         conf.FetcherTimeout(),
			logger:          l,
			action: func() error {
				return fetchDesiredState(l, clock, client, conf, store)
			},
		}

		err := ifritizeComponent(f)

		if err != nil {
			l.Error("Fetcher exited", err)
			os.Exit(197)
		}

		l.Info("exited")
		os.Exit(0)
	} else {
		err := fetchDesiredState(l, clock, client, conf, store)
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
}

func fetchDesiredState(l lager.Logger, clock clock.Clock, client httpclient.HttpClient, conf *config.Config, store store.Store) error {
	l.Info("Fetching Desired State")
	fetcher := desiredstatefetcher.New(
		conf,
		store,
		metricsaccountant.New(),
		client,
		clock,
		l,
	)

	resultChan := make(chan desiredstatefetcher.DesiredStateFetcherResult, 1)
	fetcher.Fetch(resultChan)

	result := <-resultChan

	if result.Success {
		l.Info("Success", lager.Data{"Number of Desired Apps Fetched": strconv.Itoa(result.NumResults)})
		return nil
	}

	l.Error(result.Message, result.Error)
	return result.Error
}
