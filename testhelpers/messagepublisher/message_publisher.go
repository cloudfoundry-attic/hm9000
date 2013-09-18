package messagepublisher

import (
	"encoding/json"
	"github.com/cloudfoundry/go_cfmessagebus"
	. "github.com/cloudfoundry/hm9000/models"
)

type MessagePublisher struct {
	messageBus cfmessagebus.MessageBus
}

func NewMessagePublisher(messageBus cfmessagebus.MessageBus) *MessagePublisher {
	return &MessagePublisher{
		messageBus: messageBus,
	}
}

func (publisher *MessagePublisher) PublishHeartbeat(heartbeat Heartbeat) {
	jsonResult, _ := json.Marshal(heartbeat)
	publisher.messageBus.Publish("dea.heartbeat", jsonResult)
}

func (publisher *MessagePublisher) PublishDropletExited(dropletExited DropletExitedMessage) {
	jsonResult, _ := json.Marshal(dropletExited)
	publisher.messageBus.Publish("droplet.exited", jsonResult)
}

func (publisher *MessagePublisher) PublishDropletUpdated(dropletUpdated DropletUpdatedMessage) {
	jsonResult, _ := json.Marshal(dropletUpdated)
	publisher.messageBus.Publish("droplet.updated", jsonResult)
}
