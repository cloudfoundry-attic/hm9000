package md_test

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("Crashes", func() {
	var timestamp int

	Context("when there are multiple crashed instances on a given index", func() {
		BeforeEach(func() {
			app1 := app.NewApp()

			stateServer.SetDesiredState([]models.DesiredAppState{
				app1.DesiredState(0),
			})

			timestamp = 100

			instanceHeartbeat1 := app1.GetInstance(0).Heartbeat(0)
			instanceHeartbeat1.State = models.InstanceStateCrashed
			instanceHeartbeat2 := app1.GetInstance(1).Heartbeat(0)
			instanceHeartbeat2.InstanceIndex = 0
			instanceHeartbeat2.State = models.InstanceStateCrashed

			heartbeat := models.Heartbeat{
				DeaGuid:            models.Guid(),
				InstanceHeartbeats: []models.InstanceHeartbeat{instanceHeartbeat1, instanceHeartbeat2},
			}

			for i := 0; i < 3; i++ {
				sendHeartbeats(timestamp, heartbeat)
				timestamp += 10
			}

			cliRunner.Run("fetch_desired", timestamp)
			cliRunner.Run("analyze", timestamp)
			timestamp += 30
			cliRunner.Run("send", timestamp)
		})

		It("should send a start message for the missing instance", func() {
			Ω(startStopListener.Starts).Should(BeEmpty())
			Ω(startStopListener.Stops).Should(BeEmpty())
		})
	})

})
