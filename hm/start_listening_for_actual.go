package hm

import (
	"github.com/cloudfoundry/hm9000/actualstatelistener"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/codegangsta/cli"
)

func StartListeningForActual(l logger.Logger, c *cli.Context) {
	conf := loadConfig(l, c)
	messageBus := connectToMessageBus(l, conf)
	store := connectToStore(l, conf)

	listener := actualstatelistener.New(conf,
		messageBus,
		store,
		timeprovider.NewTimeProvider(),
		l)

	listener.Start()
	l.Info("Listening for Actual State", nil)
	select {}
}
