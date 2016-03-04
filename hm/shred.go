package hm

import (
	"os"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/shredder"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/pivotal-golang/lager"
)

func Shred(l lager.Logger, conf *config.Config, poll bool) {
	store := connectToStore(l, conf)

	if poll {
		shredder := shredder.New(store, conf, l)
		err := ifritize("shredder", conf, shredder, l)

		if err != nil {
			l.Error("Shredder Exiting on error", err)
			os.Exit(197)
		}
		l.Info("Shredder Ifrit exited normally")
		os.Exit(1)
	} else {
		err := shred(l, store, conf)
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
}

func shred(l lager.Logger, store store.Store, conf *config.Config) error {
	l.Info("Shredding Store")
	theShredder := shredder.New(store, conf, l)
	return theShredder.Shred()
}
