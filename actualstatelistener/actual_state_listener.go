package actualstatelistener

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/freshnessmanager"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"

	"github.com/cloudfoundry/go_cfmessagebus"
)

type ActualStateListener struct {
	logger           logger.Logger
	config           config.Config
	messageBus       cfmessagebus.MessageBus
	heartbeatStore   store.Store
	freshnessManager freshnessmanager.FreshnessManager
	timeProvider     timeprovider.TimeProvider
}

func New(config config.Config,
	messageBus cfmessagebus.MessageBus,
	heartbeatStore store.Store,
	freshnessManager freshnessmanager.FreshnessManager,
	timeProvider timeprovider.TimeProvider,
	logger logger.Logger) *ActualStateListener {

	return &ActualStateListener{
		logger:           logger,
		config:           config,
		messageBus:       messageBus,
		heartbeatStore:   heartbeatStore,
		freshnessManager: freshnessManager,
		timeProvider:     timeProvider,
	}
}

func (listener *ActualStateListener) Start() {
	listener.messageBus.Subscribe("dea.heartbeat", func(messageBody []byte) {
		listener.bumpFreshness()

		heartbeat, err := models.NewHeartbeatFromJSON(messageBody)

		if err != nil {
			listener.logger.Info("Could not unmarshal heartbeat",
				map[string]string{
					"Error":       err.Error(),
					"MessageBody": string(messageBody),
				})
			return
		}

		nodes := make([]store.StoreNode, len(heartbeat.InstanceHeartbeats))
		for i, instance := range heartbeat.InstanceHeartbeats {
			nodes[i] = store.StoreNode{
				Key:   "/actual/" + instance.StoreKey(),
				Value: instance.ToJson(),
				TTL:   listener.config.HeartbeatTTL,
			}
		}

		err = listener.heartbeatStore.Set(nodes)

		if err != nil {
			listener.logger.Info("Could not put instance heartbeats in store:",
				map[string]string{
					"Error": err.Error(),
				})
		}
	})
}

func (listener *ActualStateListener) bumpFreshness() {
	err := listener.freshnessManager.Bump(listener.config.ActualFreshnessKey, listener.config.ActualFreshnessTTL, listener.timeProvider.Time())

	if err != nil {
		listener.logger.Info("Could not update actual freshness",
			map[string]string{
				"Error": err.Error(),
			})
	}
}
