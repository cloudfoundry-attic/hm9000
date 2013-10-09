package md_test

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Crashes", func() {
	var (
		timestamp int
		a         app.App
		heartbeat models.Heartbeat
	)

	Context("when there are multiple crashed instances on a given index", func() {
		BeforeEach(func() {
			a = app.NewApp()

			desiredState := a.DesiredState()
			desiredState.NumberOfInstances = 2
			stateServer.SetDesiredState([]models.DesiredAppState{
				desiredState,
			})

			timestamp = 100

			heartbeat = models.Heartbeat{
				DeaGuid: models.Guid(),
				InstanceHeartbeats: []models.InstanceHeartbeat{
					a.InstanceAtIndex(0).Heartbeat(),
					a.CrashedInstanceHeartbeatAtIndex(1),
					a.CrashedInstanceHeartbeatAtIndex(1),
				},
			}

			for i := 0; i < 3; i++ {
				sendHeartbeats(timestamp, heartbeat)
				timestamp += 10
			}

			cliRunner.Run("fetch_desired", timestamp)
			cliRunner.Run("analyze", timestamp)
			cliRunner.Run("send", timestamp)
		})

		It("should send a start message", func() {
			Ω(startStopListener.Stops).Should(BeEmpty())
			Ω(startStopListener.Starts).Should(HaveLen(1))
			Ω(startStopListener.Starts[0].AppVersion).Should(Equal(a.AppVersion))
			Ω(startStopListener.Starts[0].InstanceIndex).Should(Equal(1))
		})

		Context("when time passes", func() {
			BeforeEach(func() {
				timestamp += 30
				cliRunner.Run("send", timestamp)
			})

			It("should still not send any stop messages", func() {
				Ω(startStopListener.Stops).Should(BeEmpty())
			})
		})

		tick := func() {
			timestamp += 10
			sendHeartbeats(timestamp, heartbeat)
			cliRunner.Run("analyze", timestamp)
			cliRunner.Run("send", timestamp)
		}

		expireKeepAlive := func() {
			for i := 0; i < 3; i++ {
				tick()
			}
		}

		Context("when the app keeps crashing", func() {
			It("should keep restarting the app with an appropriate backoff", func() {
				//crash #2
				startStopListener.Reset()
				expireKeepAlive()
				tick()
				Ω(startStopListener.Starts).Should(HaveLen(1))

				//crash #3
				startStopListener.Reset()
				expireKeepAlive()
				tick()
				Ω(startStopListener.Starts).Should(HaveLen(1))

				//crash #4, backoff begins
				startStopListener.Reset()
				expireKeepAlive()
				tick()
				Ω(startStopListener.Starts).Should(HaveLen(0))
				tick()
				tick()
				tick()
				Ω(startStopListener.Starts).Should(HaveLen(1))
			})
		})
	})
})
