package actualstatelisteners

import (
	"os"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/nats-io/nats"

	"github.com/cloudfoundry/yagnats"
)

type NatsListener struct {
	logger     lager.Logger
	config     *config.Config
	messageBus yagnats.NATSConn
	syncer     Syncer
}

func NewNatsListener(logger lager.Logger,
	config *config.Config,
	messageBus yagnats.NATSConn,
	syncer Syncer) *NatsListener {

	return &NatsListener{
		logger:     logger,
		config:     config,
		messageBus: messageBus,
		syncer:     syncer,
	}
}

func (listener *NatsListener) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	sub, _ := listener.messageBus.Subscribe("dea.heartbeat", func(message *nats.Msg) {
		listener.logger.Debug("Got a nats heartbeat")
		heartbeat, err := models.NewHeartbeatFromJSON(message.Data)
		if err != nil {
			listener.logger.Error("Failed to unmarshal dea heartbeat", err,
				lager.Data{
					"MessageBody": string(message.Data),
				})
			return
		}
		listener.logger.Debug("Decoded the nats heartbeat")
		listener.syncer.Heartbeat(heartbeat)
	})

	close(ready)

	select {
	case <-signals:
		listener.messageBus.Unsubscribe(sub)
		return nil
	}
}
