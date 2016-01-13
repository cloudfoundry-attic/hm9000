package hm

import (
	"github.com/cloudfoundry/hm9000/analyzer"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/store"

	"os"
)

func Analyze(l logger.Logger, conf *config.Config, poll bool) {
	store := connectToStore(l, conf)

	if poll {
		l.Info("Starting Analyze Daemon...")

		adapter := connectToStoreAdapter(l, conf)
		err := Daemonize("Analyzer", func() error {
			return analyze(l, conf, store)
		}, conf.AnalyzerPollingInterval(), conf.AnalyzerTimeout(), l, adapter)

		if err != nil {
			l.Error("Analyze Daemon Errored", err)
		}
		l.Info("Analyze Daemon is Down")
		os.Exit(1)
	} else {
		err := analyze(l, conf, store)
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
}

func analyze(l logger.Logger, conf *config.Config, store store.Store) error {
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
