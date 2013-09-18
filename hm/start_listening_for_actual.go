package hm

import (
	"github.com/cloudfoundry/hm9000/actualstatelistener"
	"github.com/cloudfoundry/hm9000/helpers/freshnessmanager"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/codegangsta/cli"
)

func StartListeningForActual(l logger.Logger, c *cli.Context) {
	conf := loadConfig(l, c)
	messageBus := connectToMessageBus(l, conf)
	etcdStore := connectToETCDStore(l, conf)

	listener := actualstatelistener.New(conf,
		messageBus,
		etcdStore,
		freshnessmanager.NewFreshnessManager(etcdStore),
		timeprovider.NewTimeProvider(),
		l)

	listener.Start()
	l.Info("Listening for Actual State", nil)
	select {}
}
