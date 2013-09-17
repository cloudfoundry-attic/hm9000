package hm

import (
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/codegangsta/cli"

	"os"
)

func loadConfig(l logger.Logger, c *cli.Context) config.Config {
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

	return conf
}

func connectToMessageBus(l logger.Logger, conf config.Config) cfmessagebus.MessageBus {
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

	return messageBus
}

func connectToETCDStore(l logger.Logger, conf config.Config) store.Store {
	etcdStore := store.NewETCDStore(conf.StoreURLs, conf.StoreMaxConcurrentRequests)
	err := etcdStore.Connect()
	if err != nil {
		l.Info("Failed to connect to the store", map[string]string{"Error": err.Error()})
		os.Exit(1)
	}

	return etcdStore
}
