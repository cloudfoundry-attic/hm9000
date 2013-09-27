package startstoplistener_test

import (
	. "github.com/cloudfoundry/hm9000/testhelpers/startstoplistener"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/go_cfmessagebus/mock_cfmessagebus"
	. "github.com/cloudfoundry/hm9000/models"

	"testing"
)

func TestStartStopListener(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Start Stop Listener Suite")
}

var _ = Describe("StartStopListener", func() {
	var (
		listener       *StartStopListener
		fakeMessageBus *mock_cfmessagebus.MockMessageBus
	)

	jsonStartMessage := `{
	            "droplet":"abc",
	            "version":"123",
	            "instance_index":1,
	            "running_indices":[{"index":0, "running":1},{"index":1, "running":0}]
	        }`

	jsonStopMessage := `{
	            "droplet":"abc",
	            "version":"123",
	            "instance_guid":"xyz",
	            "instance_index":2,
	            "running_indices":[{"index":0, "running":1},{"index":1, "running":1},{"index":2, "running":2},{"index":3, "running":1}]
	        }`

	BeforeEach(func() {
		fakeMessageBus = mock_cfmessagebus.NewMockMessageBus()
		listener = NewStartStopListener(fakeMessageBus)
	})

	Describe("when a start message arrives", func() {
		It("adds a start message to its list", func() {
			expectedStart := StartMessage{
				AppGuid:       "abc",
				AppVersion:    "123",
				InstanceIndex: 1,
				RunningIndices: []RunningIndex{
					RunningIndex{0, 1},
					RunningIndex{1, 0},
				},
			}

			fakeMessageBus.PublishSync("health.start", []byte(jsonStartMessage))

			Ω(listener.Starts).Should(HaveLen(1))
			Ω(listener.Starts[0]).Should(Equal(expectedStart))
		})
	})

	Describe("when a stop message arrives", func() {
		It("adds a stop message to its list", func() {
			expectedStop := StopMessage{
				AppGuid:       "abc",
				AppVersion:    "123",
				InstanceGuid:  "xyz",
				InstanceIndex: 2,
				RunningIndices: []RunningIndex{
					RunningIndex{0, 1},
					RunningIndex{1, 1},
					RunningIndex{2, 2},
					RunningIndex{3, 1},
				},
			}

			fakeMessageBus.PublishSync("health.stop", []byte(jsonStopMessage))

			Ω(listener.Stops).Should(HaveLen(1))
			Ω(listener.Stops[0]).Should(Equal(expectedStop))
		})
	})

	Describe("reset", func() {
		It("clears out the lists", func() {
			fakeMessageBus.PublishSync("health.start", []byte(jsonStartMessage))
			fakeMessageBus.PublishSync("health.start", []byte(jsonStartMessage))

			fakeMessageBus.PublishSync("health.stop", []byte(jsonStopMessage))
			fakeMessageBus.PublishSync("health.stop", []byte(jsonStopMessage))

			Ω(listener.Starts).Should(HaveLen(2))
			Ω(listener.Stops).Should(HaveLen(2))

			listener.Reset()

			Ω(listener.Starts).Should(BeEmpty())
			Ω(listener.Stops).Should(BeEmpty())
		})
	})
})
