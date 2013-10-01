package hm

import (
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/sender"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"

	"os"
	"time"
)

func Send(l logger.Logger, conf config.Config, pollingInterval int) {
	messageBus := connectToMessageBus(l, conf)
	etcdStoreAdapter := connectToETCDStoreAdapter(l, conf)

	if pollingInterval == 0 {
		err := send(l, conf, messageBus, etcdStoreAdapter)
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	} else {
		l.Info("Starting Sender Daemon...")
		err := Daemonize(func() error {
			return send(l, conf, messageBus, etcdStoreAdapter)
		}, time.Duration(pollingInterval)*time.Second, time.Duration(pollingInterval)*10*time.Second, l)
		if err != nil {
			l.Error("Sender Daemon Errored", err)
		}
		l.Info("Sender Daemon is Down")
	}
}

func send(l logger.Logger, conf config.Config, messageBus cfmessagebus.MessageBus, etcdStoreAdapter storeadapter.StoreAdapter) error {
	store := store.NewStore(conf, etcdStoreAdapter)
	l.Info("Sending...")

	sender := sender.New(store, conf, messageBus, timeprovider.NewTimeProvider(), l)
	err := sender.Send()

	if err != nil {
		l.Error("Sender failed with error", err)
		return err
	} else {
		l.Info("Sender completed succesfully")
		return nil
	}
}
