package hm

import (
	"github.com/cloudfoundry/hm9000/analyzer"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/outbox"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"os"
)

func Analyze(l logger.Logger, conf config.Config) {
	store := connectToStore(l, conf)
	outbox := outbox.New(store, l)

	l.Info("Analyzing...", nil)

	analyzer := analyzer.New(store, outbox, timeprovider.NewTimeProvider(), conf)
	err := analyzer.Analyze()

	if err != nil {
		l.Info("Analyzer failed with error", map[string]string{"Error": err.Error()})
		os.Exit(1)
	} else {
		l.Info("Analyzer completed succesfully", nil)
		os.Exit(0)
	}
}
