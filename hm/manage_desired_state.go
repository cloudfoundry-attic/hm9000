package hm

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket"
	"github.com/cloudfoundry/hm9000/analyzer"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/desiredstatefetcher"
	"github.com/cloudfoundry/hm9000/helpers/httpclient"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/sender"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/yagnats"
	"code.cloudfoundry.org/clock"
)

type analyzeOutput struct {
	Apps          map[string]*models.App
	StartMessages []models.PendingStartMessage
	StopMessages  []models.PendingStopMessage
}

func Analyze(l lager.Logger, sink lager.Sink, conf *config.Config, poll bool) {
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
				return analyze(l, sink, clock, client, conf, store, messageBus)
			},
		}

		consulClient, _ := consuladapter.NewClientFromUrl(conf.ConsulCluster)
		lockRunner := locket.NewLock(l, consulClient, "hm9000.analyzer", make([]byte, 0), clock, locket.RetryInterval, locket.LockTTL)

		err := ifritizeComponent(f, lockRunner)

		if err != nil {
			l.Error("Analyzer exited", err)
			os.Exit(197)
		}

		l.Info("exited")
		os.Exit(0)
	} else {
		err := analyze(l, sink, clock, client, conf, store, messageBus)
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
}

func analyze(l lager.Logger, sink lager.Sink, clock clock.Clock, client httpclient.HttpClient, conf *config.Config, store store.Store, messageBus yagnats.NATSConn) error {
	logger := lager.NewLogger("fetcher")
	logger.RegisterSink(sink)

	appQueue := models.NewAppQueue()

	fetchDesiredErr := make(chan error, 1)
	go func() {
		// fetching should always block. If we are successful we mark the appQueue.fetchDesiredAppsSuccess
		// as successful. This will then signal the analyzer to know it is safe to delete apps.

		// On an error we will still need to close the channel so the analyzer does not block.
		// In this case the success flag will be false and so the analyzer should not send any updates.
		defer close(appQueue.DesiredApps)
		defer close(fetchDesiredErr)
		e := fetchDesiredState(logger, clock, client, conf, appQueue)
		fetchDesiredErr <- e
	}()

	analyzeStateErr := make(chan error, 1)
	analyzeStateOutput := make(chan analyzeOutput, 1)

	go func() {
		defer close(analyzeStateErr)
		defer close(analyzeStateOutput)
		output, e := analyzeState(l, clock, conf, store, appQueue)
		analyzeStateErr <- e
		analyzeStateOutput <- output
	}()

	var output analyzeOutput

ANALYZE_LOOP:
	for {
		select {
		case desiredErr := <-fetchDesiredErr:
			if desiredErr != nil {
				return desiredErr
			}
		case analyzeErr := <-analyzeStateErr:
			if analyzeErr != nil {
				return analyzeErr
			}
		case output = <-analyzeStateOutput:
			if output.Apps != nil {
				break ANALYZE_LOOP
			}
		}
	}

	logger = lager.NewLogger("sender")
	logger.RegisterSink(sink)
	return send(logger, conf, messageBus, store, clock, output)
}

func fetchDesiredState(l lager.Logger, clock clock.Clock, client httpclient.HttpClient, conf *config.Config, appQueue *models.AppQueue) error {
	l.Info("Fetching Desired State")
	fetcher := desiredstatefetcher.New(
		conf,
		client,
		clock,
		l,
	)

	resultChan := make(chan desiredstatefetcher.DesiredStateFetcherResult, 1)
	defer close(resultChan)

	fetcher.Fetch(resultChan, appQueue)

	result := <-resultChan

	if result.Success {
		appQueue.SetFetchDesiredAppsSuccess(true)
		l.Info("Success", lager.Data{"Number of Desired Apps Fetched": strconv.Itoa(result.NumResults)})
		return nil
	}

	l.Error(result.Message, result.Error)
	return result.Error
}

func analyzeState(l lager.Logger, clk clock.Clock, conf *config.Config, store store.Store, appQueue *models.AppQueue) (analyzeOutput, error) {
	l.Info("Analyzing...")

	t := time.Now()
	a := analyzer.New(store, clk, l, conf)
	apps, startMessages, stopMessages, err := a.Analyze(appQueue)
	analyzer.SendMetrics(apps, err)

	if err != nil {
		l.Error("Analyzer failed with error", err)
		return analyzeOutput{}, err
	}

	l.Info("Analyzer completed succesfully", lager.Data{
		"Duration": fmt.Sprintf("%.4f", time.Since(t).Seconds()),
	})
	output := analyzeOutput{
		Apps:          apps,
		StartMessages: startMessages,
		StopMessages:  stopMessages,
	}
	return output, nil
}

func send(l lager.Logger, conf *config.Config, messageBus yagnats.NATSConn, store store.Store, clock clock.Clock, outputFromAnalyze analyzeOutput) error {
	l.Info("Sending...")

	sender := sender.New(store, metricsaccountant.New(), conf, messageBus, l, clock)
	err := sender.Send(clock, outputFromAnalyze.Apps, outputFromAnalyze.StartMessages, outputFromAnalyze.StopMessages)

	if err != nil {
		l.Error("Sender failed with error", err)
		return err
	}

	l.Info("Sender completed succesfully")
	return nil
}
