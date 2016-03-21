package hm

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/sender"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/yagnats"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"

	"os"
)

func Send(l lager.Logger, conf *config.Config, poll bool) {
	messageBus := connectToMessageBus(l, conf)
	store := connectToStore(l, conf)
	clock := buildClock(l)

	if poll {
		l.Info("Starting Sender Daemon...")
		s := &Component{
			component:       "sender",
			conf:            conf,
			pollingInterval: conf.SenderPollingInterval(),
			timeout:         conf.SenderTimeout(),
			logger:          l,
			action: func() error {
				return send(l, conf, messageBus, store, clock)
			},
		}

		err := ifritizeComponent(s)

		if err != nil {
			l.Error("Sender Daemon Errored", err)
			os.Exit(197)
		}

		l.Info("Sender Daemon is Down")
		os.Exit(0)
	} else {
		err := send(l, conf, messageBus, store, clock)
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}
}

func send(l lager.Logger, conf *config.Config, messageBus yagnats.NATSConn, store store.Store, clock clock.Clock) error {
	l.Info("Sending...")

	sender := sender.New(store, metricsaccountant.New(), conf, messageBus, l, clock)
	err := sender.Send(clock)

	if err != nil {
		l.Error("Sender failed with error", err)
		return err
	}

	l.Info("Sender completed succesfully")
	return nil
}
