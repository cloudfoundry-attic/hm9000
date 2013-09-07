package message_publisher

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"
	"github.com/cloudfoundry/go_cfmessagebus/mock_cfmessagebus"
	. "github.com/cloudfoundry/hm9000/models"

	"testing"
)

func TestBootstrap(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Message Publisher")
}

var _ = Describe("MessagePublisher", func() {
	var (
		publisher      *MessagePublisher
		fakeMessageBus *mock_cfmessagebus.MockMessageBus
	)

	BeforeEach(func() {
		fakeMessageBus = mock_cfmessagebus.NewMockMessageBus()
		publisher = NewMessagePublisher(fakeMessageBus)
	})

	It("can publish hearbeats", func() {
		heartbeat := Heartbeat{
			DeaGuid: "ABC",
			InstanceHeartbeats: []InstanceHeartbeat{
				InstanceHeartbeat{
					CCPartition:    "default",
					AppGuid:        "abc",
					AppVersion:     "123",
					InstanceGuid:   "xyz",
					InstanceIndex:  1,
					State:          InstanceStateRunning,
					StateTimestamp: 1377816348.0,
				},
			},
		}

		message := make(chan []byte, 0)
		fakeMessageBus.Subscribe("dea.heartbeat", func(payload []byte) {
			message <- payload
		})

		publisher.PublishHeartbeat(heartbeat)

		var parsedJson map[string]interface{}
		err := json.Unmarshal(<-message, &parsedJson)
		Ω(err).ShouldNot(HaveOccured())

		Ω(parsedJson["dea"]).Should(Equal("ABC"))

		droplets := parsedJson["droplets"].([]interface{})
		Ω(droplets).Should(HaveLen(1))

		instanceHeartbeat := droplets[0].(map[string]interface{})
		Ω(instanceHeartbeat["cc_partition"]).Should(Equal("default"))
		Ω(instanceHeartbeat["droplet"]).Should(Equal("abc"))
		Ω(instanceHeartbeat["version"]).Should(Equal("123"))
		Ω(instanceHeartbeat["instance"]).Should(Equal("xyz"))
		Ω(instanceHeartbeat["index"]).Should(BeNumerically("==", 1))
		Ω(instanceHeartbeat["state"]).Should(Equal(string(InstanceStateRunning)))
		Ω(instanceHeartbeat["state_timestamp"]).Should(Equal(1377816348.0))
	})

	It("can publish droplet exited", func() {
		droplet_exited := DropletExitedMessage{
			CCPartition:     "default",
			AppGuid:         "abc",
			AppVersion:      "123",
			InstanceGuid:    "xyz",
			InstanceIndex:   1,
			Reason:          DropletExitedReasonStopped,
			ExitStatusCode:  0,
			ExitDescription: "stopped",
		}

		message := make(chan []byte, 0)
		fakeMessageBus.Subscribe("droplet.exited", func(payload []byte) {
			message <- payload
		})

		publisher.PublishDropletExited(droplet_exited)

		var parsedJson map[string]interface{}
		err := json.Unmarshal(<-message, &parsedJson)
		Ω(err).ShouldNot(HaveOccured())

		Ω(parsedJson["cc_partition"]).Should(Equal("default"))
		Ω(parsedJson["droplet"]).Should(Equal("abc"))
		Ω(parsedJson["version"]).Should(Equal("123"))
		Ω(parsedJson["instance"]).Should(Equal("xyz"))
		Ω(parsedJson["index"]).Should(BeNumerically("==", 1))
		Ω(parsedJson["reason"]).Should(Equal(string(DropletExitedReasonStopped)))
		Ω(parsedJson["exit_status"]).Should(BeNumerically("==", 0))
		Ω(parsedJson["exit_description"]).Should(Equal("stopped"))
		Ω(parsedJson["crash_timestamp"]).Should(BeNil())
	})

	It("can publish droplet exited in crashed case", func() {
		droplet_exited := DropletExitedMessage{
			CCPartition:     "default",
			AppGuid:         "abc",
			AppVersion:      "123",
			InstanceGuid:    "xyz",
			InstanceIndex:   1,
			Reason:          DropletExitedReasonCrashed,
			ExitStatusCode:  0,
			ExitDescription: "kaboom!",
			CrashTimestamp:  17,
		}

		message := make(chan []byte, 0)
		fakeMessageBus.Subscribe("droplet.exited", func(payload []byte) {
			message <- payload
		})

		publisher.PublishDropletExited(droplet_exited)

		var parsedJson map[string]interface{}
		err := json.Unmarshal(<-message, &parsedJson)
		Ω(err).ShouldNot(HaveOccured())

		Ω(parsedJson["cc_partition"]).Should(Equal("default"))
		Ω(parsedJson["droplet"]).Should(Equal("abc"))
		Ω(parsedJson["version"]).Should(Equal("123"))
		Ω(parsedJson["instance"]).Should(Equal("xyz"))
		Ω(parsedJson["index"]).Should(BeNumerically("==", 1))
		Ω(parsedJson["reason"]).Should(Equal(string(DropletExitedReasonCrashed)))
		Ω(parsedJson["exit_status"]).Should(BeNumerically("==", 0))
		Ω(parsedJson["exit_description"]).Should(Equal("kaboom!"))
		Ω(parsedJson["crash_timestamp"]).Should(BeNumerically("==", 17))
	})

	It("can publish droplet updated", func() {
		droplet_updated := DropletUpdatedMessage{
			AppGuid: "abc",
		}

		message := make(chan []byte, 0)
		fakeMessageBus.Subscribe("droplet.updated", func(payload []byte) {
			message <- payload
		})

		publisher.PublishDropletUpdated(droplet_updated)

		var parsedJson map[string]interface{}
		err := json.Unmarshal(<-message, &parsedJson)
		Ω(err).ShouldNot(HaveOccured())

		Ω(parsedJson["droplet"]).Should(Equal("abc"))
	})

})
