package hm

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/apiserver"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
)

func ServeAPI(l logger.Logger, conf config.Config) {
	store := connectToStore(l, conf)

	apiServer := apiserver.New(
		conf.APIServerPort,
		store,
		buildTimeProvider(l),
	)

	go apiServer.Start()
	l.Info(fmt.Sprintf("Serving API on port %d", conf.APIServerPort))
	select {}
}
