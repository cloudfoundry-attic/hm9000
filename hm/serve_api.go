package hm

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/cloudfoundry-incubator/natbeat"
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

	members := grouper.Members{
		{"api", http_server.New(listenAddr, handler)},
	}

	natsAddresses := []string{}

	for _, natsAddress := range conf.NATS {
		natsAddresses = append(natsAddresses, fmt.Sprintf("%s:%d", natsAddress.Host, natsAddress.Port))
	}

	registration := initializeServerRegistration(l, conf)

	members = append(members, grouper.Member{
		Name:   "background_heartbeat",
		Runner: natbeat.NewBackgroundHeartbeat(strings.Join(natsAddresses, ","), conf.NATS[0].User, conf.NATS[0].Password, &LagerAdapter{l}, registration),
	})

	group := grouper.NewOrdered(os.Interrupt, members)

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

func initializeServerRegistration(l logger.Logger, conf *config.Config) (registration natbeat.RegistryMessage) {
	uri, err := url.Parse(conf.APIServerURL)
	if err != nil {
		l.Error("cannot parse url", err)
		os.Exit(1)
	}

	return natbeat.RegistryMessage{
		URIs: []string{uri.Host},
		Host: conf.APIServerAddress,
		Port: conf.APIServerPort,
	}
}
