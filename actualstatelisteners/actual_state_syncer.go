package actualstatelisteners

import (
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

const HeartbeatSyncTimer = "HeartbeatSyncTimer"

//go:generate counterfeiter -o fakes/fake_syncer.go . Syncer
type Syncer interface {
	Run(signals <-chan os.Signal, ready chan<- struct{}) error
	Heartbeat(heartbeat *models.Heartbeat)
}

type actualStateSyncer struct {
	logger            lager.Logger
	config            *config.Config
	store             store.Store
	clock             clock.Clock
	storeUsageTracker metricsaccountant.UsageTracker
	metricsAccountant metricsaccountant.MetricsAccountant
	heartbeatsToSave  []*models.Heartbeat

	heartbeatMutex *sync.Mutex
}

func NewSyncer(logger lager.Logger,
	config *config.Config,
	store store.Store,
	storeUsageTracker metricsaccountant.UsageTracker,
	metricsAccountant metricsaccountant.MetricsAccountant,
	clock clock.Clock) Syncer {

	return &actualStateSyncer{
		logger:            logger,
		config:            config,
		store:             store,
		storeUsageTracker: storeUsageTracker,
		metricsAccountant: metricsAccountant,
		clock:             clock,
		heartbeatsToSave:  []*models.Heartbeat{},
		heartbeatMutex:    &sync.Mutex{},
	}
}

func (syncer *actualStateSyncer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	syncCtl := make(chan bool)
	go syncer.syncHeartbeats(syncCtl)

	syncer.storeUsageTracker.StartTrackingUsage()

	close(ready)

	select {
	case <-signals:
		syncCtl <- true
		return nil
	}
}

func (syncer *actualStateSyncer) Heartbeat(heartbeat *models.Heartbeat) {
	syncer.heartbeatMutex.Lock()

	syncer.heartbeatsToSave = append(syncer.heartbeatsToSave, heartbeat)
	numToSave := len(syncer.heartbeatsToSave)

	syncer.heartbeatMutex.Unlock()

	syncer.logger.Info("Received a heartbeat", lager.Data{
		"Heartbeats Pending Save": strconv.Itoa(numToSave),
	})

	// send start here?
}

func (syncer *actualStateSyncer) syncHeartbeats(ctlChan <-chan bool) {
	syncInterval := syncer.clock.NewTicker(syncer.config.ListenerHeartbeatSyncInterval())

	for {
		syncer.heartbeatMutex.Lock()
		heartbeatsToSave := syncer.heartbeatsToSave
		syncer.heartbeatsToSave = []*models.Heartbeat{}
		syncer.heartbeatMutex.Unlock()

		numHeartbeats := len(heartbeatsToSave)

		if numHeartbeats > 0 {
			syncer.logger.Info("Saving Heartbeats", lager.Data{
				"Heartbeats to Save": strconv.Itoa(numHeartbeats),
			})

			t := syncer.clock.Now()
			err := syncer.store.SyncHeartbeats(heartbeatsToSave...)

			if err != nil {
				syncer.logger.Error("Could not put instance heartbeats in store:", err)
				syncer.store.RevokeActualFreshness()
			} else {
				dt := syncer.clock.Since(t)
				if dt < syncer.config.ListenerHeartbeatSyncInterval() {
					syncer.bumpFreshness()
				} else {
					syncer.logger.Info("Save took too long.  Not bumping freshness.")
				}
				syncer.logger.Info("Saved Heartbeats", lager.Data{
					"Heartbeats to Save": strconv.Itoa(numHeartbeats),
					"Duration":           time.Since(t).String(),
				})

				syncer.heartbeatMutex.Lock()
				syncer.heartbeatMutex.Unlock()

				syncer.metricsAccountant.TrackSavedHeartbeats(numHeartbeats)
			}
		}

		syncer.logger.Debug("Tracking Heartbeat Metrics", lager.Data{
			"Total Received Heartbeats in Interval": strconv.Itoa(numHeartbeats),
		})
		t := syncer.clock.Now()

		syncer.metricsAccountant.TrackReceivedHeartbeats(numHeartbeats)

		syncer.logger.Debug("Done Tracking Heartbeat Metrics", lager.Data{
			"Total Received Heartbeats in Interval": strconv.Itoa(numHeartbeats),
			"Duration":                              time.Since(t).String(),
		})

		select {
		case <-ctlChan:
			break
		case <-syncInterval.C():
		}
	}
}

func (syncer *actualStateSyncer) bumpFreshness() {
	err := syncer.store.BumpActualFreshness(syncer.clock.Now())
	if err != nil {
		syncer.logger.Error("Could not update actual freshness", err)
	} else {
		syncer.logger.Info("Bumped freshness")
	}
}
