package hm

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/yagnats"
	"github.com/nats-io/nats"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"

	"os"
)

func buildClock(l lager.Logger) clock.Clock {
	if os.Getenv("HM9000_FAKE_TIME") == "" {
		return clock.NewClock()
	} else {
		timestamp, err := strconv.Atoi(os.Getenv("HM9000_FAKE_TIME"))
		if err != nil {
			l.Error("Failed to load timestamp", err)
			os.Exit(1)
		}
		return NewFixedClock(time.Unix(int64(timestamp), 0))
	}
}

func connectToMessageBus(l lager.Logger, conf *config.Config) yagnats.NATSConn {
	members := make([]string, len(conf.NATS))

	for _, natsConf := range conf.NATS {
		uri := url.URL{
			Scheme: "nats",
			User:   url.UserPassword(natsConf.User, natsConf.Password),
			Host:   fmt.Sprintf("%s:%d", natsConf.Host, natsConf.Port),
		}
		members = append(members, uri.String())
	}

	natsClient, err := yagnats.Connect(members)
	if err != nil {
		l.Error("Failed to connect to the message bus", err)
		os.Exit(1)
	}

	natsClient.AddReconnectedCB(func(conn *nats.Conn) {
		l.Info(fmt.Sprintf("NATS Client Reconnected. Server URL: %s", conn.Opts.Url))
	})

	natsClient.AddClosedCB(func(conn *nats.Conn) {
		err := fmt.Errorf("NATS Client Closed. nats.Conn: %+v", conn)
		l.Error("NATS Closed", err)
		os.Exit(1)
	})

	return natsClient
}

func connectToStoreAdapter(l lager.Logger, conf *config.Config) storeadapter.StoreAdapter {
	var adapter storeadapter.StoreAdapter
	workPool, err := workpool.NewWorkPool(conf.StoreMaxConcurrentRequests)
	if err != nil {
		l.Error("Failed to create workpool", err)
		os.Exit(1)
	}

	options := &etcdstoreadapter.ETCDOptions{
		ClusterUrls: conf.StoreURLs,
	}
	adapter, err = etcdstoreadapter.New(options, workPool)
	if err != nil {
		l.Error("Failed to create the store adapter", err)
		os.Exit(1)
	}

	err = adapter.Connect()
	if err != nil {
		l.Error("Failed to connect to the store", err)
		os.Exit(1)
	}

	return adapter
}

func connectToStore(l lager.Logger, conf *config.Config) store.Store {
	adapter := connectToStoreAdapter(l, conf)
	return store.NewStore(conf, adapter, l)
}

func connectToStoreAndTrack(l lager.Logger, conf *config.Config) (store.Store, metricsaccountant.UsageTracker) {
	tracker := newUsageTracker(conf.StoreMaxConcurrentRequests)
	adapter := connectToStoreAdapter(l, conf)
	return store.NewStore(conf, adapter, l), tracker
}

type fixedClock struct {
	now   time.Time
	clock clock.Clock
}

func NewFixedClock(now time.Time) clock.Clock {
	return &fixedClock{
		now:   now,
		clock: clock.NewClock(),
	}
}

func (fixed *fixedClock) Now() time.Time {
	return fixed.now
}

func (fixed *fixedClock) Since(t time.Time) time.Duration {
	return fixed.now.Sub(t)
}

func (fixed *fixedClock) Sleep(d time.Duration) {
	<-fixed.clock.NewTimer(d).C()
}

func (fixed *fixedClock) NewTimer(d time.Duration) clock.Timer {
	return fixed.clock.NewTimer(d)
}

func (fixed *fixedClock) NewTicker(d time.Duration) clock.Ticker {
	return fixed.clock.NewTicker(d)
}
