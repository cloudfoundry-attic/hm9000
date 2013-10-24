package md_test

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Evacuation", func() {
	var app appfixture.AppFixture

	BeforeEach(func() {
		app = appfixture.NewAppFixture()
		simulator.SetCurrentHeartbeats(app.Heartbeat(1))
		simulator.SetDesiredState(app.DesiredState(1))
		simulator.Tick(simulator.TicksToAttainFreshness)
	})

	Context("when an evacuation message comes in", func() {
		BeforeEach(func() {
			cliRunner.StartEvacuator(simulator.currentTimestamp)
			coordinator.MessageBus.Publish("droplet.exited", string(app.InstanceAtIndex(0).DropletExited(models.DropletExitedReasonDEAEvacuation).ToJSON()))
		})

		AfterEach(func() {
			cliRunner.StopEvacuator()
		})

		It("should immediately start the app", func() {
			simulator.Tick(1)
			Ω(startStopListener.Starts).Should(HaveLen(1))
			Ω(startStopListener.Starts[0].AppGuid).Should(Equal(app.AppGuid))
			Ω(startStopListener.Starts[0].AppVersion).Should(Equal(app.AppVersion))
			Ω(startStopListener.Starts[0].InstanceIndex).Should(Equal(0))
		})
	})
})
