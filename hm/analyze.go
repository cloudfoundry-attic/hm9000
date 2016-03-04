package hm

import (
	"github.com/cloudfoundry/hm9000/analyzer"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/pivotal-golang/lager"

	"os"
)

func Analyze(l lager.Logger, conf *config.Config, poll bool) {
	store := connectToStore(l, conf)

	if poll {
		l.Info("Starting Analyze Daemon...")

		a := analyzer.New(store, buildClock(l), l, conf)

		err := ifritize("analyzer", conf, a, l)

		if err != nil {
			l.Error("Analyzer exited", err)
			os.Exit(197)
		}

		l.Info("exited")
		os.Exit(0)
	} else {
		err := analyze(l, conf, store)
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
}

func analyze(l lager.Logger, conf *config.Config, store store.Store) error {
	l.Info("Analyzing...")

	a := analyzer.New(store, buildClock(l), l, conf)
	apps, err := a.Analyze()
	analyzer.SendMetrics(apps, err)

	if err != nil {
		l.Error("Analyzer failed with error", err)
		return err
	} else {
		l.Info("Analyzer completed succesfully")
		return nil
	}
}
