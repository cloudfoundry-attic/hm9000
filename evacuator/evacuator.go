package evacuator

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/yagnats"
	"github.com/nats-io/nats"
	"github.com/pivotal-golang/clock"
)

type Evacuator struct {
	messageBus yagnats.NATSConn
	store      store.Store
	clock      clock.Clock
	config     *config.Config
	logger     logger.Logger
}

func New(messageBus yagnats.NATSConn, store store.Store, clock clock.Clock, config *config.Config, logger logger.Logger) *Evacuator {
	return &Evacuator{
		messageBus: messageBus,
		store:      store,
		clock:      clock,
		config:     config,
		logger:     logger,
	}
}

func (e *Evacuator) Listen() {
	e.messageBus.Subscribe("droplet.exited", func(message *nats.Msg) {
		dropletExited, err := models.NewDropletExitedFromJSON([]byte(message.Data))
		if err != nil {
			e.logger.Error("Failed to parse droplet exited message", err)
			return
		}

		e.handleExited(dropletExited)
	})
}

func (e *Evacuator) handleExited(exited models.DropletExited) {
	switch exited.Reason {
	case models.DropletExitedReasonDEAShutdown, models.DropletExitedReasonDEAEvacuation:
		startMessage := models.NewPendingStartMessage(
			e.clock.Now(),
			0,
			e.config.GracePeriod(),
			exited.AppGuid,
			exited.AppVersion,
			exited.InstanceIndex,
			2.0,
			models.PendingStartMessageReasonEvacuating,
		)
		startMessage.SkipVerification = true

		e.logger.Info("Scheduling start message for droplet.exited message", startMessage.LogDescription(), exited.LogDescription())

		e.store.SavePendingStartMessages(startMessage)
	}
}
