package hm

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/cloudfoundry/hm9000/analyzer"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/desiredstatefetcher"
	"github.com/cloudfoundry/hm9000/helpers/httpclient"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/sender"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/yagnats"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

func Analyze(l lager.Logger, conf *config.Config, poll bool) {
	store := connectToStore(l, conf)
	messageBus := connectToMessageBus(l, conf)
	clock := buildClock(l)
	client := httpclient.NewHttpClient(conf.SkipSSLVerification, conf.FetcherNetworkTimeout())

	if poll {
		l.Info("Starting Analyzer...")

		f := &Component{
			component:       "analyzer",
			conf:            conf,
			pollingInterval: conf.FetcherPollingInterval(),
			timeout:         conf.FetcherTimeout(),
			logger:          l,
			action: func() error {
				return analyze(l, clock, client, conf, store, messageBus)
			},
		}

		err := ifritizeComponent(f)

		if err != nil {
			l.Error("Analyzer exited", err)
			os.Exit(197)
		}

		l.Info("exited")
		os.Exit(0)
	} else {
		err := analyze(l, clock, client, conf, store, messageBus)
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
}

func analyze(l lager.Logger, clock clock.Clock, client httpclient.HttpClient, conf *config.Config, store store.Store, messageBus yagnats.NATSConn) error {
	e := fetchDesiredState(l, clock, client, conf, store)

	if e != nil {
		return e
	}

	e = analyzeState(l, clock, conf, store)
	if e != nil {
		return e
	}

	return send(l, conf, messageBus, store, clock)
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

func analyzeState(l lager.Logger, clk clock.Clock, conf *config.Config, store store.Store) error {
	l.Info("Analyzing...")

	t := time.Now()
	a := analyzer.New(store, clk, l, conf)
	apps, err := a.Analyze()
	analyzer.SendMetrics(apps, err)

	if err != nil {
		l.Error("Analyzer failed with error", err)
		return err
	}

	l.Info("Analyzer completed succesfully", lager.Data{
		"Duration": fmt.Sprintf("%.4f", time.Since(t).Seconds()),
	})
	return nil
}

func send(l lager.Logger, conf *config.Config, messageBus yagnats.NATSConn, store store.Store, clock clock.Clock) error {
	l.Info("Sending...")

	sender := sender.New(store, metricsaccountant.New(), conf, messageBus, l, clock)
	err := sender.Send(clock)

	if err != nil {
		l.Error("Sender failed with error", err)
		return err
	}

	l.Info("Sender completed succesfully")
	return nil
}
