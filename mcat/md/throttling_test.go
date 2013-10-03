package md_test

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Sending messages in batches", func() {
	//Note: the sender is configured to only send 8 messages at a time
	//This is done by cli_runner_test.go when it generates the config
	Context("when there are start and stop messages", func() {
		var timestamp int

		BeforeEach(func() {
			var desiredStates = []models.DesiredAppState{}
			var heartbeats = []models.Heartbeat{}
			for i := 0; i < 40; i += 1 {
				appToStart := app.NewApp()
				appToStop := app.NewApp()
				desiredStates = append(desiredStates, appToStart.DesiredState(0))
				heartbeats = append(heartbeats, appToStop.Heartbeat(1, 0))
			}
			stateServer.SetDesiredState(desiredStates)
			timestamp = 100

			for i := 0; i < 3; i++ {
				sendHeartbeats(timestamp, heartbeats...)
				timestamp += 10
			}
			cliRunner.Run("fetch_desired", timestamp)
			cliRunner.Run("analyze", timestamp)
			for i := 0; i < 3; i++ {
				sendHeartbeats(timestamp, heartbeats...)
				timestamp += 10
			}
			cliRunner.Run("send", timestamp)
		})

		It("should send all the stops", func() {
			Ω(startStopListener.Stops).Should(HaveLen(40))
		})

		It("should send up to the limit # of starts", func() {
			Ω(startStopListener.Starts).Should(HaveLen(8))
		})

		Context("when told to send again", func() {
			var firstBatch []models.StartMessage
			BeforeEach(func() {
				firstBatch = startStopListener.Starts
				startStopListener.Reset()
				cliRunner.Run("send", timestamp)
			})

			It("should send the next batch of starts", func() {
				Ω(startStopListener.Starts).Should(HaveLen(8))
				for _, message := range firstBatch {
					Ω(startStopListener.Starts).ShouldNot(ContainElement(message))
				}
			})

			It("should not send anymore stops (as they were all sent)", func() {
				Ω(startStopListener.Stops).Should(BeEmpty())
			})
		})
	})
})
