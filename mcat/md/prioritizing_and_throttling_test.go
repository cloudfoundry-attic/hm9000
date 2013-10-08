package md_test

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Prioritizing and sending messages in batches", func() {
	//Note: the sender is configured to only send 8 messages at a time
	//This is done by cli_runner_test.go when it generates the config
	Context("when there are start and stop messages", func() {
		var timestamp int
		var highPriorityAppGuids []string
		var mediumPriorityAppGuids []string
		var lowPriorityAppGuids []string

		BeforeEach(func() {
			var desiredStates = []models.DesiredAppState{}
			var heartbeats = []models.Heartbeat{}

			lowPriorityAppGuids = make([]string, 0)
			//this generates 8 low priority start messages
			for i := 0; i < 8; i += 1 {
				appToStart := app.NewApp()
				desiredState := appToStart.DesiredState()
				desiredState.NumberOfInstances = 4
				desiredStates = append(desiredStates, desiredState)
				lowPriorityAppGuids = append(lowPriorityAppGuids, appToStart.AppGuid)
				heartbeats = append(heartbeats, appToStart.Heartbeat(3))
			}

			mediumPriorityAppGuids = make([]string, 0)
			//this generates 8 medium priority start messages
			for i := 0; i < 8; i += 1 {
				appToStart := app.NewApp()
				desiredState := appToStart.DesiredState()
				desiredState.NumberOfInstances = 2
				desiredStates = append(desiredStates, desiredState)
				mediumPriorityAppGuids = append(mediumPriorityAppGuids, appToStart.AppGuid)
				heartbeats = append(heartbeats, appToStart.Heartbeat(1))
			}

			highPriorityAppGuids = make([]string, 0)
			//this generates 9 high priority start messages
			for i := 0; i < 9; i += 1 {
				appToStart := app.NewApp()
				desiredState := appToStart.DesiredState()
				desiredState.NumberOfInstances = 1
				desiredStates = append(desiredStates, desiredState)
				highPriorityAppGuids = append(highPriorityAppGuids, appToStart.AppGuid)
			}

			for i := 0; i < 40; i += 1 {
				appToStop := app.NewApp()
				heartbeats = append(heartbeats, appToStop.Heartbeat(1))
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

		It("should send up to the limit # of starts and should send the high priority starts first", func() {
			Ω(startStopListener.Starts).Should(HaveLen(8))
			for _, startMessage := range startStopListener.Starts {
				Ω(highPriorityAppGuids).Should(ContainElement(startMessage.AppGuid))
			}
		})

		Context("when told to send again", func() {
			BeforeEach(func() {
				startStopListener.Reset()
				cliRunner.Run("send", timestamp)
			})

			It("should send the next batch of starts", func() {
				Ω(startStopListener.Starts).Should(HaveLen(8))
				for i, startMessage := range startStopListener.Starts {
					if i == 0 {
						Ω(highPriorityAppGuids).Should(ContainElement(startMessage.AppGuid))
					} else {
						Ω(mediumPriorityAppGuids).Should(ContainElement(startMessage.AppGuid))
					}
				}
			})

			It("should not send anymore stops (as they were all sent)", func() {
				Ω(startStopListener.Stops).Should(BeEmpty())
			})
		})
	})
})
