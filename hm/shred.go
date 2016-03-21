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
		s := &Component{
			component:       "shredder",
			conf:            conf,
			pollingInterval: conf.ShredderPollingInterval(),
			timeout:         conf.ShredderTimeout(),
			logger:          l,
			action: func() error {
				return shred(l, store)
			},
		}

		err := ifritizeComponent(s)

		if err != nil {
			l.Error("Shredder Exiting on error", err)
			os.Exit(197)
		}
		l.Info("Shredder Ifrit exited normally")
		os.Exit(1)
	} else {
		err := shred(l, store)
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
}

func shred(l lager.Logger, store store.Store) error {
	l.Info("Shredding Store")
	theShredder := shredder.New(store, l)
	return theShredder.Shred()
}
