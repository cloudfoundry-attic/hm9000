package hm

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/metricsserver"
)

func ServeMetrics(steno *gosteno.Logger, l logger.Logger, conf config.Config) {
	store := connectToStore(l, conf)

	metricsServer := metricsserver.New(
		steno,
		store,
		buildTimeProvider(l),
		conf,
	)

	l.Info("Serving Metrics")
	err := metricsServer.Start()
	if err != nil {
		l.Error("Failed to serve metrics", err)
	}
}
