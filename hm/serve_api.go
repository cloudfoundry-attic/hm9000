package hm

import (
	"fmt"
	"os"

	"github.com/cloudfoundry/hm9000/apiserver/handlers"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"

	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

func ServeAPI(l logger.Logger, conf *config.Config) {
	store, _ := connectToStore(l, conf)

	apiHandler, err := handlers.New(l, store, buildTimeProvider(l))
	if err != nil {
		l.Error("initialize-handler.failed", err)
		panic(err)
	}
	handler := handlers.BasicAuthWrap(apiHandler, conf.APIServerUsername, conf.APIServerPassword)

	listenAddr := fmt.Sprintf("%s:%d", conf.APIServerAddress, conf.APIServerPort)
	group := grouper.NewOrdered(os.Interrupt, grouper.Members{
		{"api", http_server.New(listenAddr, handler)},
	})

	monitor := ifrit.Invoke(sigmon.New(group))

	l.Info("started")
	l.Info(listenAddr)

	err = <-monitor.Wait()
	if err != nil {
		l.Error("exited", err)
		os.Exit(1)
	}

	l.Info("exited")
	os.Exit(0)
}
