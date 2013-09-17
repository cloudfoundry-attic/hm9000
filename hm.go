package main

import (
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/desiredstatefetcher"
	"github.com/cloudfoundry/hm9000/helpers/bel_air"
	"github.com/cloudfoundry/hm9000/helpers/http_client"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/time_provider"
	"github.com/cloudfoundry/hm9000/store"
	"strconv"
	"time"

	"github.com/codegangsta/cli"

	"os"
)

func main() {
	l := logger.NewRealLogger()

	app := cli.NewApp()
	app.Name = "HM9000"
	app.Usage = "Start the various HM9000 components"
	app.Commands = []cli.Command{
		cli.Command{
			Name:        "fetch_desired",
			Description: "Fetches desired state",
			Usage:       "hm fetch_desired --config=/path/to/config",
			Flags: []cli.Flag{
				cli.StringFlag{"config", "", "Path to config file"},
			},
			Action: func(c *cli.Context) {
				fetchDesiredState(l, c)
			},
		},
	}

	app.Run(os.Args)
}

func fetchDesiredState(l logger.Logger, c *cli.Context) {
	configPath := c.String("config")
	if configPath == "" {
		l.Info("Config path required", nil)
		os.Exit(1)
	}

	conf, err := config.FromFile(configPath)
	if err != nil {
		l.Info("Failed to load config", map[string]string{"Error": err.Error()})
		os.Exit(1)
	}

	messageBus, err := cfmessagebus.NewMessageBus("NATS")
	if err != nil {
		l.Info("Failed to initialize the message bus", map[string]string{"Error": err.Error()})
		os.Exit(1)
	}

	messageBus.Configure(conf.NATS.Host, conf.NATS.Port, conf.NATS.User, conf.NATS.Password)
	err = messageBus.Connect()
	if err != nil {
		l.Info("Failed to connect to the message bus", map[string]string{"Error": err.Error()})
		os.Exit(1)
	}

	etcdStore := store.NewETCDStore(config.ETCD_URL(4001))
	err = etcdStore.Connect()
	if err != nil {
		l.Info("Failed to connect to the store", map[string]string{"Error": err.Error()})
		os.Exit(1)
	}

	fetcher := desiredstatefetcher.NewDesiredStateFetcher(conf,
		messageBus,
		etcdStore,
		http_client.NewHttpClient(),
		bel_air.NewFreshPrince(etcdStore),
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
