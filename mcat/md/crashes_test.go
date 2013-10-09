package md_test

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Crashes", func() {
	var (
		a         app.App
	)

	Context("when there are multiple crashed instances on a given index", func() {
		BeforeEach(func() {
			a = app.NewApp()
			desiredState := a.DesiredState()
			desiredState.NumberOfInstances = 2
			simulator.SetDesiredState(desiredState)

			simulator.SetCurrentHeartbeats( models.Heartbeat{
				DeaGuid: models.Guid(),
				InstanceHeartbeats: []models.InstanceHeartbeat{
					a.InstanceAtIndex(0).Heartbeat(),
					a.CrashedInstanceHeartbeatAtIndex(1),
					a.CrashedInstanceHeartbeatAtIndex(1),
				},
			})
			simulator.Tick(simulator.TicksToAttainFreshness)
			simulator.Tick(1)
		})

		It("should send a start message", func() {
			Ω(startStopListener.Stops).Should(BeEmpty())
			Ω(startStopListener.Starts).Should(HaveLen(1))
			Ω(startStopListener.Starts[0].AppVersion).Should(Equal(a.AppVersion))
			Ω(startStopListener.Starts[0].InstanceIndex).Should(Equal(1))
		})

		Context("when time passes", func() {
			BeforeEach(func() {
				simulator.Tick(1)
			})

			It("should still not send any stop messages", func() {
				Ω(startStopListener.Stops).Should(BeEmpty())
			})
		})

		Context("when the app keeps crashing", func() {
			It("should keep restarting the app with an appropriate backoff", func() {
				//crash #2
				startStopListener.Reset()
				simulator.Tick(simulator.TicksToAttainFreshness)
				simulator.Tick(1)
				Ω(startStopListener.Starts).Should(HaveLen(1))

				//crash #3
				startStopListener.Reset()
				simulator.Tick(simulator.TicksToAttainFreshness)
				simulator.Tick(1)
				Ω(startStopListener.Starts).Should(HaveLen(1))

				//crash #4, backoff begins
				startStopListener.Reset()
				simulator.Tick(simulator.TicksToAttainFreshness)
				simulator.Tick(1)
				Ω(startStopListener.Starts).Should(HaveLen(0))

				//take more ticks longer to send a new start messages
				simulator.Tick(simulator.TicksToAttainFreshness)
				Ω(startStopListener.Starts).Should(HaveLen(1))
			})
		})
	})
})
