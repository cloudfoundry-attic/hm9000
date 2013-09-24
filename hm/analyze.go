package hm

import (
	"github.com/cloudfoundry/hm9000/analyzer"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/outbox"
	"os"

	"github.com/codegangsta/cli"
)

func Analyze(l logger.Logger, c *cli.Context) {
	conf := loadConfig(l, c)
	store := connectToStore(l, conf)
	outbox := outbox.NewLoggingOutbox(l)

	l.Info("Analyzing...", nil)

	analyzer := analyzer.New(store, outbox)
	err := analyzer.Analyze()

	if err != nil {
		l.Info("Analyzer failed with error", map[string]string{"Error": err.Error()})
		os.Exit(1)
	} else {
		l.Info("Analyzer completed succesfully", nil)
		os.Exit(0)
	}
}
