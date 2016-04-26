package mcat_test

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Prioritizing and sending messages in batches", func() {
	//Note: the sender is configured to only send 8 messages at a time
	//This is done by cli_runner_test.go when it generates the config
	Context("when there are start and stop messages", func() {
		var highPriorityAppGuids []string
		var lowPriorityAppGuids []string

		BeforeEach(func() {
			var desiredStates = []models.DesiredAppState{}
			var heartbeats = []*models.Heartbeat{}

			lowPriorityAppGuids = make([]string, 0)
			for i := 0; i < 8; i += 1 {
				appToStart := appfixture.NewAppFixture()
				desiredState := appToStart.DesiredState(2)
				desiredStates = append(desiredStates, desiredState)
				lowPriorityAppGuids = append(lowPriorityAppGuids, appToStart.AppGuid)
				heartbeats = append(heartbeats, appToStart.Heartbeat(1))
			}

			highPriorityAppGuids = make([]string, 0)
			for i := 0; i < 9; i += 1 {
				appToStart := appfixture.NewAppFixture()
				desiredState := appToStart.DesiredState(1)
				desiredStates = append(desiredStates, desiredState)
				highPriorityAppGuids = append(highPriorityAppGuids, appToStart.AppGuid)
			}

			for i := 0; i < 40; i += 1 {
				appToStop := appfixture.NewAppFixture()
				heartbeats = append(heartbeats, appToStop.Heartbeat(1))
			}

			simulator.SetDesiredState(desiredStates...)
			simulator.SetCurrentHeartbeats(heartbeats...)
			simulator.Tick(simulator.TicksToAttainFreshness)
			simulator.Tick(simulator.GracePeriod)
		})

		It("should send all the stops", func() {
			Expect(startStopListener.StopCount()).To(Equal(40))
		})

		It("should send up to the limit # of starts with the highest priorities first", func() {
			Expect(startStopListener.StartCount()).To(Equal(8))
			for i := 0; i < 8; i++ {
				startMessage := startStopListener.Start(i)
				Expect(highPriorityAppGuids).To(ContainElement(startMessage.AppGuid))
			}
		})
	})
})
