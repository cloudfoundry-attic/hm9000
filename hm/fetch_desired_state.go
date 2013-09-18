package hm

import (
	"github.com/cloudfoundry/hm9000/desiredstatefetcher"
	"github.com/cloudfoundry/hm9000/helpers/freshnessmanager"
	"github.com/cloudfoundry/hm9000/helpers/http_client"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/time_provider"
	"github.com/codegangsta/cli"

	"os"
	"strconv"
	"time"
)

func FetchDesiredState(l logger.Logger, c *cli.Context) {
	conf := loadConfig(l, c)
	messageBus := connectToMessageBus(l, conf)
	etcdStore := connectToETCDStore(l, conf)

	fetcher := desiredstatefetcher.New(conf,
		messageBus,
		etcdStore,
		http_client.NewHttpClient(),
		freshnessmanager.NewFreshnessManager(etcdStore),
		time_provider.NewTimeProvider(),
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
