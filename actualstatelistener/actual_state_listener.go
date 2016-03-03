package actualstatelistener

import (
	"strconv"
	"sync"
	"time"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/nats-io/nats"
	"github.com/pivotal-golang/clock"

	"github.com/cloudfoundry/yagnats"
)

const HeartbeatSyncTimer = "HeartbeatSyncTimer"

type ActualStateListener struct {
	logger                  logger.Logger
	config                  *config.Config
	messageBus              yagnats.NATSConn
	store                   store.Store
	clock                   clock.Clock
	storeUsageTracker       metricsaccountant.UsageTracker
	metricsAccountant       metricsaccountant.MetricsAccountant
	heartbeatsToSave        []models.Heartbeat
	totalReceivedHeartbeats int
	totalSavedHeartbeats    int

	heartbeatMutex *sync.Mutex
}

func New(config *config.Config,
	messageBus yagnats.NATSConn,
	store store.Store,
	storeUsageTracker metricsaccountant.UsageTracker,
	metricsAccountant metricsaccountant.MetricsAccountant,
	clock clock.Clock,
	logger logger.Logger) *ActualStateListener {

	return &ActualStateListener{
		logger:            logger,
		config:            config,
		messageBus:        messageBus,
		store:             store,
		storeUsageTracker: storeUsageTracker,
		metricsAccountant: metricsAccountant,
		clock:             clock,
		heartbeatsToSave:  []models.Heartbeat{},
		heartbeatMutex:    &sync.Mutex{},
	}
}

func (listener *ActualStateListener) Start() {
	listener.messageBus.Subscribe("dea.heartbeat", func(message *nats.Msg) {
		listener.logger.Debug("Got a heartbeat")
		heartbeat, err := models.NewHeartbeatFromJSON(message.Data)
		if err != nil {
			listener.logger.Error("Could not unmarshal heartbeat", err,
				map[string]string{
					"MessageBody": string(message.Data),
				})
			return
		}

		listener.logger.Debug("Decoded the heartbeat")

		listener.heartbeatMutex.Lock()

		listener.totalReceivedHeartbeats++

		listener.heartbeatsToSave = append(listener.heartbeatsToSave, heartbeat)
		numToSave := len(listener.heartbeatsToSave)

		listener.heartbeatMutex.Unlock()

		listener.logger.Info("Received a heartbeat", map[string]string{
			"Heartbeats Pending Save": strconv.Itoa(numToSave),
		})
	})

	go listener.syncHeartbeats()

	listener.storeUsageTracker.StartTrackingUsage()
	listener.measureStoreUsage()
}

func (listener *ActualStateListener) syncHeartbeats() {
	syncInterval := listener.clock.NewTicker(listener.config.ListenerHeartbeatSyncInterval())
	previousReceivedHeartbeats := -1

	for {
		listener.heartbeatMutex.Lock()
		heartbeatsToSave := listener.heartbeatsToSave
		listener.heartbeatsToSave = []models.Heartbeat{}
		totalReceivedHeartbeats := listener.totalReceivedHeartbeats
		listener.heartbeatMutex.Unlock()

		if len(heartbeatsToSave) > 0 {
			listener.logger.Info("Saving Heartbeats", map[string]string{
				"Heartbeats to Save": strconv.Itoa(len(heartbeatsToSave)),
			})

			t := listener.clock.Now()
			err := listener.store.SyncHeartbeats(heartbeatsToSave...)

			if err != nil {
				listener.logger.Error("Could not put instance heartbeats in store:", err)
				listener.store.RevokeActualFreshness()
			} else {
				dt := listener.clock.Since(t)
				if dt < listener.config.ListenerHeartbeatSyncInterval() {
					listener.bumpFreshness()
				} else {
					listener.logger.Info("Save took too long.  Not bumping freshness.")
				}
				listener.logger.Info("Saved Heartbeats", map[string]string{
					"Heartbeats to Save": strconv.Itoa(len(heartbeatsToSave)),
					"Duration":           time.Since(t).String(),
				})

				listener.heartbeatMutex.Lock()
				listener.totalSavedHeartbeats += len(heartbeatsToSave)
				totalSavedHeartbeats := listener.totalSavedHeartbeats
				listener.heartbeatMutex.Unlock()

				listener.metricsAccountant.TrackSavedHeartbeats(totalSavedHeartbeats)
			}
		}

		if previousReceivedHeartbeats != totalReceivedHeartbeats {
			listener.logger.Debug("Tracking Heartbeat Metrics", map[string]string{
				"Total Received Heartbeats": strconv.Itoa(totalReceivedHeartbeats),
			})
			t := listener.clock.Now()

			listener.metricsAccountant.TrackReceivedHeartbeats(totalReceivedHeartbeats)

			listener.logger.Debug("Done Tracking Heartbeat Metrics", map[string]string{
				"Total Received Heartbeats": strconv.Itoa(totalReceivedHeartbeats),
				"Duration":                  time.Since(t).String(),
			})

			previousReceivedHeartbeats = totalReceivedHeartbeats
		}

		<-syncInterval.C()
	}
}

func (listener *ActualStateListener) measureStoreUsage() {
	usage, _ := listener.storeUsageTracker.MeasureUsage()
	listener.metricsAccountant.TrackActualStateListenerStoreUsageFraction(usage)

	time.AfterFunc(3*time.Duration(listener.config.HeartbeatPeriod)*time.Second, func() {
		listener.measureStoreUsage()
	})
}

func (listener *ActualStateListener) bumpFreshness() {
	err := listener.store.BumpActualFreshness(listener.clock.Now())
	if err != nil {
		listener.logger.Error("Could not update actual freshness", err)
	} else {
		listener.logger.Info("Bumped freshness")
	}
}
