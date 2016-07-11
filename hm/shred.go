package hm

import (
	"os"

	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/shredder"
	"github.com/cloudfoundry/hm9000/store"
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

		consulClient, _ := consuladapter.NewClientFromUrl(conf.ConsulCluster)
		lockRunner := locket.NewLock(l, consulClient, "hm9000.shredder", make([]byte, 0), buildClock(l), locket.RetryInterval, locket.LockTTL)

		err := ifritizeComponent(s, lockRunner)

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
