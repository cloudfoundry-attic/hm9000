package mcat_test

import (
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Expiring Heartbeats Test", func() {
	var dea1, dea2 appfixture.DeaFixture
	var app1, app2, app3 appfixture.AppFixture

	BeforeEach(func() {
		dea1 = appfixture.NewDeaFixture()
		dea2 = appfixture.NewDeaFixture()

		app1 = dea1.GetApp(0)
		app2 = dea1.GetApp(1)
		app3 = dea2.GetApp(2)

		simulator.SetCurrentHeartbeats(
			dea1.HeartbeatWith(app1.InstanceAtIndex(0).Heartbeat(), app2.InstanceAtIndex(0).Heartbeat()),
			dea2.HeartbeatWith(app3.InstanceAtIndex(0).Heartbeat()),
		)
		simulator.SetDesiredState(app1.DesiredState(1), app2.DesiredState(1), app3.DesiredState(1))
		simulator.Tick(simulator.TicksToAttainFreshness, false)
	})

	Context("when a dea reports than an instance is no longer present", func() {
		BeforeEach(func() {
			simulator.SetCurrentHeartbeats(
				dea1.HeartbeatWith(app1.InstanceAtIndex(0).Heartbeat()),
				dea2.HeartbeatWith(app3.InstanceAtIndex(0).Heartbeat()),
			)
		})

		It("should start the instance after a grace period", func() {
			simulator.Tick(simulator.GracePeriod, false)
			Expect(startStopListener.StartCount()).To(Equal(0))
			simulator.Tick(1, false)
			Expect(startStopListener.StartCount()).To(Equal(1))
			Expect(startStopListener.Start(0).AppGuid).To(Equal(app2.AppGuid))

			Expect(startStopListener.StopCount()).To(Equal(0))
		})
	})

	Context("when the a dea stops reporting", func() {
		BeforeEach(func() {
			simulator.SetCurrentHeartbeats(
				dea2.HeartbeatWith(app3.InstanceAtIndex(0).Heartbeat()),
			)
		})

		It("should start all the instances on that dea after two grace periods (one to see the app is gone, the other to wait for it not to return)", func() {
			simulator.Tick(simulator.GracePeriod, false)
			Expect(startStopListener.StartCount()).To(Equal(0))
			simulator.Tick(simulator.GracePeriod, false)
			Expect(startStopListener.StartCount()).To(Equal(2))

			appGuids := []string{
				startStopListener.Start(0).AppGuid,
				startStopListener.Start(1).AppGuid,
			}
			Expect(appGuids).To(ContainElement(app1.AppGuid))
			Expect(appGuids).To(ContainElement(app2.AppGuid))

			Expect(startStopListener.StopCount()).To(Equal(0))
		})
	})
})
