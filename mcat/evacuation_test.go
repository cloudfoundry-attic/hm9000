package mcat_test

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Evacuation and Shutdown", func() {
	var dea appfixture.DeaFixture
	var app appfixture.AppFixture

	BeforeEach(func() {
		dea = appfixture.NewDeaFixture()
		app = dea.GetApp(0)
		simulator.SetCurrentHeartbeats(dea.HeartbeatWith(app.InstanceAtIndex(0).Heartbeat()))
		simulator.SetDesiredState(app.DesiredState(1))
		simulator.Tick(simulator.TicksToAttainFreshness)
	})

	Describe("Shutdown handling by the evacuator component", func() {
		Context("when a SHUTDOWN droplet.exited message comes in", func() {
			BeforeEach(func() {
				cliRunner.StartEvacuator(simulator.currentTimestamp)
				coordinator.MessageBus.Publish("droplet.exited", app.InstanceAtIndex(0).DropletExited(models.DropletExitedReasonDEAShutdown).ToJSON())
			})

			AfterEach(func() {
				cliRunner.StopEvacuator()
			})

			It("should immediately start the app", func() {
				simulator.Tick(1)
				Expect(startStopListener.StartCount()).To(Equal(1))
				startMsg := startStopListener.Start(0)
				Expect(startMsg.AppGuid).To(Equal(app.AppGuid))
				Expect(startMsg.AppVersion).To(Equal(app.AppVersion))
				Expect(startMsg.InstanceIndex).To(Equal(0))
			})
		})
	})

	Describe("Deterministic evacuation", func() {
		Context("when an app enters the evacuation state", func() {
			var evacuatingHeartbeat models.InstanceHeartbeat

			BeforeEach(func() {
				Expect(startStopListener.StartCount()).To(BeZero())
				Expect(startStopListener.StopCount()).To(BeZero())
				evacuatingHeartbeat = app.InstanceAtIndex(0).Heartbeat()
				evacuatingHeartbeat.State = models.InstanceStateEvacuating

				simulator.SetCurrentHeartbeats(dea.HeartbeatWith(evacuatingHeartbeat))
				simulator.Tick(1)
			})

			It("should immediately start the app", func() {
				Expect(startStopListener.StartCount()).To(Equal(1))
				startMsg := startStopListener.Start(0)
				Expect(startMsg.AppGuid).To(Equal(app.AppGuid))
				Expect(startMsg.AppVersion).To(Equal(app.AppVersion))
				Expect(startMsg.InstanceIndex).To(Equal(0))
				Expect(startStopListener.StopCount()).To(BeZero())
			})

			Context("when the app starts", func() {
				BeforeEach(func() {
					startStopListener.Reset()
					runningHeartbeat := app.InstanceAtIndex(0).Heartbeat()
					runningHeartbeat.InstanceGuid = models.Guid()
					simulator.SetCurrentHeartbeats(dea.HeartbeatWith(evacuatingHeartbeat))
					simulator.SetCurrentHeartbeats(models.Heartbeat{
						DeaGuid:            "new-dea",
						InstanceHeartbeats: []models.InstanceHeartbeat{runningHeartbeat},
					})
					simulator.Tick(1)
				})

				It("should stop the evacuated instance", func() {
					Expect(startStopListener.StartCount()).To(BeZero())
					Expect(startStopListener.StopCount()).To(Equal(1))
					stopMsg := startStopListener.Stop(0)
					Expect(stopMsg.AppGuid).To(Equal(app.AppGuid))
					Expect(stopMsg.AppVersion).To(Equal(app.AppVersion))
					Expect(stopMsg.InstanceGuid).To(Equal(evacuatingHeartbeat.InstanceGuid))
				})
			})
		})
	})
})
