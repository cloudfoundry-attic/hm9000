package hm

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/shredder"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"os"
)

func Shred(l logger.Logger, conf config.Config, poll bool) {
	if conf.StoreType == "Cassandra" {
		l.Info("No Shredder neccessary for Cassandra")
		select {}
	}
	adapter := connectToStoreAdapter(l, conf)

	if poll {
		l.Info("Starting Shredder Daemon...")
		err := Daemonize("Shredder", func() error {
			return shred(l, conf, adapter)
		}, conf.ShredderPollingInterval(), conf.ShredderTimeout(), l)
		if err != nil {
			l.Error("Shredder Errored", err)
		}
		l.Info("Shredder Daemon is Down")
	} else {
		err := shred(l, conf, adapter)
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
}

func shred(l logger.Logger, conf config.Config, adapter storeadapter.StoreAdapter) error {
	l.Info("Shredding Store")
	theShredder := shredder.New(adapter, l)
	return theShredder.Shred()
}
