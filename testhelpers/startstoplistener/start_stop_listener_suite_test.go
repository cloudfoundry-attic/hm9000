package startstoplistener_test

import (
	"github.com/cloudfoundry/hm9000/config"
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
		conf           config.Config
		listener       *StartStopListener
		fakeMessageBus *mock_cfmessagebus.MockMessageBus
	)

	jsonStartMessage := `{
	            "droplet":"abc",
	            "version":"123",
	            "instance_index":1
	        }`

	jsonStopMessage := `{
	            "droplet":"abc",
	            "version":"123",
	            "instance_guid":"xyz",
	            "instance_index":2,
	            "is_duplicate":true
	        }`

	BeforeEach(func() {
		conf, _ = config.DefaultConfig()
		fakeMessageBus = mock_cfmessagebus.NewMockMessageBus()
		listener = NewStartStopListener(fakeMessageBus, conf)
	})

	Describe("when a start message arrives", func() {
		It("adds a start message to its list", func() {
			expectedStart := StartMessage{
				AppGuid:       "abc",
				AppVersion:    "123",
				InstanceIndex: 1,
			}

			fakeMessageBus.PublishSync(conf.SenderNatsStartSubject, []byte(jsonStartMessage))

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
				IsDuplicate:   true,
			}

			fakeMessageBus.PublishSync(conf.SenderNatsStopSubject, []byte(jsonStopMessage))

			Ω(listener.Stops).Should(HaveLen(1))
			Ω(listener.Stops[0]).Should(Equal(expectedStop))
		})
	})

	Describe("reset", func() {
		It("clears out the lists", func() {
			fakeMessageBus.PublishSync(conf.SenderNatsStartSubject, []byte(jsonStartMessage))
			fakeMessageBus.PublishSync(conf.SenderNatsStartSubject, []byte(jsonStartMessage))

			fakeMessageBus.PublishSync(conf.SenderNatsStopSubject, []byte(jsonStopMessage))
			fakeMessageBus.PublishSync(conf.SenderNatsStopSubject, []byte(jsonStopMessage))

			Ω(listener.Starts).Should(HaveLen(2))
			Ω(listener.Stops).Should(HaveLen(2))

			listener.Reset()

			Ω(listener.Starts).Should(BeEmpty())
			Ω(listener.Stops).Should(BeEmpty())
		})
	})
})
