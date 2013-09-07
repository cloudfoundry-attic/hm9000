package actual_state_listener

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"

	"github.com/cloudfoundry/go_cfmessagebus"

	"encoding/json"
	"strconv"
)

type ActualStateListener struct {
	messageBus     cfmessagebus.MessageBus
	heartbeatStore store.Store
	timeProvider   helpers.TimeProvider
}

func NewActualStateListener(messageBus cfmessagebus.MessageBus, heartbeatStore store.Store, timeProvider helpers.TimeProvider) *ActualStateListener {
	return &ActualStateListener{
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
			//TODO:LOG!
			return
		}

		for _, instance := range heartbeat.InstanceHeartbeats {
			key := "/actual/" + instance.AppGuid + "-" + instance.AppVersion + "/" + strconv.Itoa(instance.InstanceIndex) + "/" + instance.InstanceGuid
			value, err := json.Marshal(instance)
			if err != nil {
				//TODO:LOG!
				continue
			}
			err = listener.heartbeatStore.Set(key, string(value), config.HEARTBEAT_TTL)
			if err != nil {
				//TODO:LOG!
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
		//TODO:LOG!
	}
}
