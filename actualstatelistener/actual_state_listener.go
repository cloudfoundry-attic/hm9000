package actualstatelistener

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"

	"github.com/cloudfoundry/go_cfmessagebus"
)

type ActualStateListener struct {
	logger       logger.Logger
	config       config.Config
	messageBus   cfmessagebus.MessageBus
	store        store.Store
	timeProvider timeprovider.TimeProvider
}

func New(config config.Config,
	messageBus cfmessagebus.MessageBus,
	store store.Store,
	timeProvider timeprovider.TimeProvider,
	logger logger.Logger) *ActualStateListener {

	return &ActualStateListener{
		logger:       logger,
		config:       config,
		messageBus:   messageBus,
		store:        store,
		timeProvider: timeProvider,
	}
}

func (listener *ActualStateListener) Start() {
	listener.messageBus.Subscribe("dea.advertise", func(messageBody []byte) {
		listener.logger.Info("Received dea.advertise, bumping freshness.")
		listener.bumpFreshness()
	})

	listener.messageBus.Subscribe("dea.heartbeat", func(messageBody []byte) {
		heartbeat, err := models.NewHeartbeatFromJSON(messageBody)
		if err != nil {
			listener.logger.Error("Could not unmarshal heartbeat", err,
				map[string]string{
					"MessageBody": string(messageBody),
				})
			return
		}

		err = listener.store.SaveActualState(heartbeat.InstanceHeartbeats...)
		if err != nil {
			listener.logger.Error("Could not put instance heartbeats in store:", err)
			return
		}

		listener.logger.Info("Received dea.heartbeat, bumping freshness.")
		listener.bumpFreshness()
	})
}

func (listener *ActualStateListener) bumpFreshness() {
	err := listener.store.BumpActualFreshness(listener.timeProvider.Time())
	if err != nil {
		listener.logger.Error("Could not update actual freshness", err)
	}
}
