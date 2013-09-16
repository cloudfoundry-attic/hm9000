package actualstatelistener

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/bel_air"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/time_provider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"

	"github.com/cloudfoundry/go_cfmessagebus"
)

type ActualStateListener struct {
	logger.Logger
	config         config.Config
	messageBus     cfmessagebus.MessageBus
	heartbeatStore store.Store
	freshPrince    bel_air.FreshPrince
	timeProvider   time_provider.TimeProvider
}

func NewActualStateListener(config config.Config,
	messageBus cfmessagebus.MessageBus,
	heartbeatStore store.Store,
	freshPrince bel_air.FreshPrince,
	timeProvider time_provider.TimeProvider,
	logger logger.Logger) *ActualStateListener {

	return &ActualStateListener{
		Logger:         logger,
		config:         config,
		messageBus:     messageBus,
		heartbeatStore: heartbeatStore,
		freshPrince:    freshPrince,
		timeProvider:   timeProvider,
	}
}

func (listener *ActualStateListener) Start() {
	listener.messageBus.Subscribe("dea.heartbeat", func(messageBody []byte) {
		listener.bumpFreshness()

		heartbeat, err := models.NewHeartbeatFromJSON(messageBody)

		if err != nil {
			listener.Info("Could not unmarshal heartbeat from store",
				map[string]string{
					"Error":       err.Error(),
					"MessageBody": string(messageBody),
				})
			return
		}

		for _, instance := range heartbeat.InstanceHeartbeats {
			key := "/actual/" + instance.InstanceGuid
			value := instance.ToJson()
			err = listener.heartbeatStore.Set(key, value, listener.config.HeartbeatTTL)

			if err != nil {
				listener.Info("Could not put instance heartbeat in store:",
					map[string]string{
						"Error":     err.Error(),
						"Heartbeat": string(value),
					})
			}
		}
	})
}

func (listener *ActualStateListener) bumpFreshness() {
	err := listener.freshPrince.Bump(listener.config.ActualFreshnessKey, listener.config.ActualFreshnessTTL, listener.timeProvider.Time())

	if err != nil {
		listener.Info("Could not update actual freshness",
			map[string]string{
				"Error": err.Error(),
			})
	}
}
