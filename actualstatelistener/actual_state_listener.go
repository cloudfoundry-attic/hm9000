package actualstatelistener

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/time_provider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"

	"github.com/cloudfoundry/go_cfmessagebus"

	"encoding/json"
)

type ActualStateListener struct {
	logger.Logger
	messageBus     cfmessagebus.MessageBus
	heartbeatStore store.Store
	timeProvider   time_provider.TimeProvider
}

func NewActualStateListener(messageBus cfmessagebus.MessageBus,
	heartbeatStore store.Store,
	timeProvider time_provider.TimeProvider,
	logger logger.Logger) *ActualStateListener {

	return &ActualStateListener{
		Logger:         logger,
		messageBus:     messageBus,
		heartbeatStore: heartbeatStore,
		timeProvider:   timeProvider,
	}
}

func (listener *ActualStateListener) Start() {
	listener.messageBus.Subscribe("dea.heartbeat", func(messageBody []byte) {
		listener.bumpFreshness()

		var heartbeat models.Heartbeat
		err := json.Unmarshal(messageBody, &heartbeat)

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

			value, err := json.Marshal(instance)
			if err != nil {
				listener.Info("Could not json Marshal instance",
					map[string]string{
						"Error": err.Error(),
					})
				continue
			}

			err = listener.heartbeatStore.Set(key, string(value), config.HEARTBEAT_TTL)
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
	var jsonTimestamp string
	oldTimestamp, err := listener.heartbeatStore.Get(config.ACTUAL_FRESHNESS_KEY)

	if err == nil {
		jsonTimestamp = oldTimestamp.Value
	} else {
		now := listener.timeProvider.Time().Unix()
		jsonBytes, _ := json.Marshal(models.FreshnessTimestamp{Timestamp: now})
		jsonTimestamp = string(jsonBytes)
	}

	err = listener.heartbeatStore.Set(config.ACTUAL_FRESHNESS_KEY, jsonTimestamp, config.ACTUAL_FRESHNESS_TTL)

	if err != nil {
		listener.Info("Could not update actual freshness",
			map[string]string{
				"Error": err.Error(),
			})
	}
}
