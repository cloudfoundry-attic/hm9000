package hm

import (
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/metricsserver"
)

func ServeMetrics(steno *gosteno.Logger, l logger.Logger, conf config.Config) {
	store := connectToStore(l, conf)
	mbus := connectToMessageBus(l, conf)

	metricsServer := metricsserver.New(
		mbus,
		steno,
		store,
		buildTimeProvider(l),
		conf,
	)

	err := metricsServer.Start()
	if err != nil {
		l.Error("Failed to serve metrics", err)
	}
	l.Info("Serving Metrics")
	select {}
}
