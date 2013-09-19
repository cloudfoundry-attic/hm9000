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
	            "last_updated":1377816348,
	            "version":"123",
	            "indices":[1,2],
	            "running":{"123":2},
	            "flapping":false
	        }`

	jsonStopMessage := `{
	            "droplet":"abc",
	            "last_updated":1377816348,
	            "version":"123",
	            "instances":{"xyz":"123", "uvw":"123"},
	            "running":{"123":2}
	        }`

	BeforeEach(func() {
		fakeMessageBus = mock_cfmessagebus.NewMockMessageBus()
		listener = NewStartStopListener(fakeMessageBus)
	})

	Describe("when a start message arrives", func() {
		It("adds a start message to its list", func() {
			expectedStart := StartMessage{
				AppGuid:                   "abc",
				LastUpdated:               1377816348,
				AppVersion:                "123",
				IndicesToStart:            []int{1, 2},
				RunningInstancesByVersion: map[string]int{"123": 2},
				Flapping:                  false,
			}

			fakeMessageBus.PublishSync("health.start", []byte(jsonStartMessage))

			Ω(listener.Starts).Should(HaveLen(1))
			Ω(listener.Starts[0]).Should(Equal(expectedStart))
		})
	})

	Describe("when a stop message arrives", func() {
		It("adds a stop message to its list", func() {
			expectedStop := StopMessage{
				AppGuid:                   "abc",
				LastUpdated:               1377816348,
				AppVersion:                "123",
				InstancesToVersion:        map[string]string{"xyz": "123", "uvw": "123"},
				RunningInstancesByVersion: map[string]int{"123": 2},
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
