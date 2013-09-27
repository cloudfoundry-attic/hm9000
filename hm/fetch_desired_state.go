package hm

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/desiredstatefetcher"
	"github.com/cloudfoundry/hm9000/helpers/httpclient"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"

	"os"
	"strconv"
	"time"
)

func FetchDesiredState(l logger.Logger, conf config.Config) {
	messageBus := connectToMessageBus(l, conf)
	store := connectToStore(l, conf)

	fetcher := desiredstatefetcher.New(conf,
		messageBus,
		store,
		httpclient.NewHttpClient(),
		timeprovider.NewTimeProvider(),
	)

	resultChan := make(chan desiredstatefetcher.DesiredStateFetcherResult, 1)
	fetcher.Fetch(resultChan)

	select {
	case result := <-resultChan:
		if result.Success {
			l.Info("Success", map[string]string{"Number of Desired Apps Fetched": strconv.Itoa(result.NumResults)})
			os.Exit(0)
		} else {
			l.Info(result.Message, map[string]string{"Error": result.Error.Error(), "Message": result.Message})
			os.Exit(1)
		}
	case <-time.After(600 * time.Second):
		l.Info("Timed out when fetching desired state", nil)
		os.Exit(1)
	}
}
