package actualstatelistener

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"time"

	"github.com/cloudfoundry/yagnats"
)

type ActualStateListener struct {
	logger            logger.Logger
	config            config.Config
	messageBus        yagnats.NATSClient
	store             store.Store
	timeProvider      timeprovider.TimeProvider
	storeUsageTracker metricsaccountant.UsageTracker
	metricsAccountant metricsaccountant.MetricsAccountant
}

func New(config config.Config,
	messageBus yagnats.NATSClient,
	store store.Store,
	storeUsageTracker metricsaccountant.UsageTracker,
	metricsAccountant metricsaccountant.MetricsAccountant,
	timeProvider timeprovider.TimeProvider,
	logger logger.Logger) *ActualStateListener {

	return &ActualStateListener{
		logger:            logger,
		config:            config,
		messageBus:        messageBus,
		store:             store,
		storeUsageTracker: storeUsageTracker,
		metricsAccountant: metricsAccountant,
		timeProvider:      timeProvider,
	}
}

func (listener *ActualStateListener) Start() {
	listener.messageBus.Subscribe("dea.advertise", func(message *yagnats.Message) {
		listener.bumpFreshness()
		listener.logger.Debug("Received dea.advertise")
	})

	listener.messageBus.Subscribe("dea.heartbeat", func(message *yagnats.Message) {
		listener.logger.Debug("Got a heartbeat")
		heartbeat, err := models.NewHeartbeatFromJSON([]byte(message.Payload))
		if err != nil {
			listener.logger.Error("Could not unmarshal heartbeat", err,
				map[string]string{
					"MessageBody": message.Payload,
				})
			return
		}

		listener.logger.Debug("Decoded the heartbeat")
		err = listener.store.SyncHeartbeat(heartbeat)
		if err != nil {
			listener.logger.Error("Could not put instance heartbeats in store:", err)
			return
		}

		listener.logger.Info("Saved a Heartbeat", heartbeat.LogDescription())
		listener.bumpFreshness()
		listener.logger.Debug("Received dea.heartbeat") //Leave this here: the integration test uses this to ensure the heartbeat has been processed
	})

	if listener.storeUsageTracker != nil {
		listener.storeUsageTracker.StartTrackingUsage()
		listener.measureStoreUsage()
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
	err := listener.store.BumpActualFreshness(listener.timeProvider.Time())
	if err != nil {
		listener.logger.Error("Could not update actual freshness", err)
	} else {
		listener.logger.Info("Bumped freshness")
	}
}
