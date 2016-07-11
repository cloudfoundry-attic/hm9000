package hm

import (
	"os"

	"code.cloudfoundry.org/consuladapter"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/locket"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/evacuator"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
)

func StartEvacuator(logger lager.Logger, conf *config.Config) {
	messageBus := connectToMessageBus(logger, conf)
	store := connectToStore(logger, conf)

	clock := buildClock(logger)

	evac := evacuator.New(messageBus, store, clock, metricsaccountant.New(), conf, logger)

	consulClient, _ := consuladapter.NewClientFromUrl(conf.ConsulCluster)
	lockRunner := locket.NewLock(logger, consulClient, "hm9000.evacuator", make([]byte, 0), clock, locket.RetryInterval, locket.LockTTL)

	err := ifritize(logger, "evacuator", evac, conf, lockRunner)
	if err != nil {
		logger.Error("exited", err)
		os.Exit(197)
	}

	logger.Info("exited")
	os.Exit(0)
}
