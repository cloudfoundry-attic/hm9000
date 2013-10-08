package md_test

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Crashes", func() {
	var timestamp int

	Context("when there are multiple crashed instances on a given index", func() {
		BeforeEach(func() {
			a := app.NewApp()

			stateServer.SetDesiredState([]models.DesiredAppState{
				a.DesiredState(),
			})

			timestamp = 100

			heartbeat := models.Heartbeat{
				DeaGuid: models.Guid(),
				InstanceHeartbeats: []models.InstanceHeartbeat{
					a.CrashedInstanceHeartbeatAtIndex(0),
					a.CrashedInstanceHeartbeatAtIndex(0),
				},
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
