package hm

import (
	"fmt"
	"time"

	"github.com/cloudfoundry/hm9000/analyzer"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"

	"os"
)

func Analyze(l lager.Logger, conf *config.Config, poll bool) {
	store := connectToStore(l, conf)
	clock := buildClock(l)

	if poll {
		l.Info("Starting Analyze Daemon...")
		a := &Component{
			component:       "analyzer",
			conf:            conf,
			pollingInterval: conf.AnalyzerPollingInterval(),
			timeout:         conf.AnalyzerTimeout(),
			logger:          l,
			action: func() error {
				return analyze(l, clock, conf, store)
			},
		}

		err := ifritizeComponent(a)

		if err != nil {
			l.Error("Analyzer exited", err)
			os.Exit(197)
		}

		l.Info("exited")
		os.Exit(0)
	} else {
		err := analyze(l, clock, conf, store)
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
}

func analyze(l lager.Logger, clk clock.Clock, conf *config.Config, store store.Store) error {
	l.Info("Analyzing...")

	t := time.Now()
	a := analyzer.New(store, clk, l, conf)
	apps, err := a.Analyze()
	analyzer.SendMetrics(apps, err)

	if err != nil {
		l.Error("Analyzer failed with error", err)
		return err
	}

	l.Info("Analyzer completed succesfully", lager.Data{
		"Duration": fmt.Sprintf("%.4f", time.Since(t).Seconds()),
	})
	return nil
}
