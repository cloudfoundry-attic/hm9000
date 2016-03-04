package hm

import (
	"os"

	"github.com/cloudfoundry/hm9000/config"
	evacuatorpackage "github.com/cloudfoundry/hm9000/evacuator"
	"github.com/pivotal-golang/lager"
)

func StartEvacuator(logger lager.Logger, conf *config.Config) {
	messageBus := connectToMessageBus(logger, conf)
	store := connectToStore(logger, conf)

	clock := buildClock(logger)

	evacuator := evacuatorpackage.New(messageBus, store, clock, conf, logger)

	err := ifritize("evacuator", conf, evacuator, logger)
	if err != nil {
		logger.Error("exited", err)
		os.Exit(197)
	}

	logger.Info("exited")
	os.Exit(0)
}
