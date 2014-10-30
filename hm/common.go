package hm

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/cloudfoundry/gunk/timeprovider"
	"github.com/cloudfoundry/gunk/timeprovider/faketimeprovider"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/yagnats"

	"os"
)

func buildTimeProvider(l logger.Logger) timeprovider.TimeProvider {
	if os.Getenv("HM9000_FAKE_TIME") == "" {
		return timeprovider.NewTimeProvider()
	} else {
		timestamp, err := strconv.Atoi(os.Getenv("HM9000_FAKE_TIME"))
		if err != nil {
			l.Error("Failed to load timestamp", err)
			os.Exit(1)
		}
		return &faketimeprovider.FakeTimeProvider{
			TimeToProvide: time.Unix(int64(timestamp), 0),
		}
	}
}

func connectToMessageBus(l logger.Logger, conf *config.Config) yagnats.NATSConn {
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

	return natsClient
}

func acquireLock(l logger.Logger, conf *config.Config, lockName string) {
	adapter := connectToStoreAdapter(l, conf, nil)
	l.Info("Acquiring lock for " + lockName)

	lock := storeadapter.StoreNode{
		Key: "/hm/locks/" + lockName,
		TTL: 10,
	}

	status, _, err := adapter.MaintainNode(lock)
	if err != nil {
		l.Error("Failed to talk to lock store", err)
		os.Exit(1)
	}

	lockAcquired := make(chan bool)

	go func() {
		for {
			if <-status {
				if lockAcquired != nil {
					close(lockAcquired)
					lockAcquired = nil
				}
			} else {
				l.Error("Lost the lock", errors.New("Lost the lock"))
				os.Exit(197)
			}
		}
	}()

	<-lockAcquired
	l.Info("Acquired lock for " + lockName)
}

func connectToStoreAdapter(l logger.Logger, conf *config.Config, usage *usageTracker) storeadapter.StoreAdapter {
	var adapter storeadapter.StoreAdapter
	var around workpool.AroundWork = workpool.DefaultAround
	if usage != nil {
		around = usage
	}
	workPool := workpool.New(conf.StoreMaxConcurrentRequests, 0, around)
	adapter = etcdstoreadapter.NewETCDStoreAdapter(conf.StoreURLs, workPool)
	err := adapter.Connect()
	if err != nil {
		l.Error("Failed to connect to the store", err)
		os.Exit(1)
	}

	return adapter
}

func connectToStore(l logger.Logger, conf *config.Config) store.Store {
	adapter := connectToStoreAdapter(l, conf, nil)
	return store.NewStore(conf, adapter, l)
}

func connectToStoreAndTrack(l logger.Logger, conf *config.Config) (store.Store, metricsaccountant.UsageTracker) {
	tracker := newUsageTracker(conf.StoreMaxConcurrentRequests)
	adapter := connectToStoreAdapter(l, conf, tracker)
	return store.NewStore(conf, adapter, l), tracker
}
