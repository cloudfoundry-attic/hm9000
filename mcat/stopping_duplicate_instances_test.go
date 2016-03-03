package mcat_test

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stopping Duplicate Instances", func() {
	var dea appfixture.DeaFixture
	var a appfixture.AppFixture

	Context("when there are multiple instances on the same index", func() {
		var instance0, instance1, duplicateInstance1 appfixture.Instance
		var heartbeat models.Heartbeat
		BeforeEach(func() {
			dea = appfixture.NewDeaFixture()
			a = dea.GetApp(0)

			instance0 = a.InstanceAtIndex(0)
			instance1 = a.InstanceAtIndex(1)
			duplicateInstance1 = a.InstanceAtIndex(1)
			duplicateInstance1.InstanceGuid = models.Guid()

			heartbeat = dea.HeartbeatWith(instance0.Heartbeat(), instance1.Heartbeat(), duplicateInstance1.Heartbeat())
			simulator.SetCurrentHeartbeats(heartbeat)

			simulator.SetDesiredState(a.DesiredState(2))

			simulator.Tick(simulator.TicksToAttainFreshness)
		})

		It("should not immediately stop anything", func() {
			Expect(startStopListener.StopCount()).To(Equal(0))
		})

		Context("after four grace periods", func() {
			Context("if both instances are still running", func() {
				BeforeEach(func() {
					simulator.Tick(simulator.GracePeriod * 4)
				})

				It("should stop one of them", func() {
					Expect(startStopListener.StopCount()).To(Equal(1))
					stop := startStopListener.Stop(0)
					Expect(stop.AppGuid).To(Equal(a.AppGuid))
					Expect(stop.AppVersion).To(Equal(a.AppVersion))
					Expect(stop.InstanceIndex).To(Equal(1))
					Expect(stop.IsDuplicate).To(BeTrue())
					Expect([]string{instance1.InstanceGuid, duplicateInstance1.InstanceGuid}).To(ContainElement(stop.InstanceGuid))
				})

				Context("after another grace period (assuming the stopped instance stops)", func() {
					BeforeEach(func() {
						instanceGuidThatShouldStop := startStopListener.Stop(0).InstanceGuid

						var remainingInstance appfixture.Instance
						if instance1.InstanceGuid == instanceGuidThatShouldStop {
							remainingInstance = duplicateInstance1
						} else {
							remainingInstance = instance1
						}

						heartbeat = dea.HeartbeatWith(instance0.Heartbeat(), remainingInstance.Heartbeat())
						simulator.SetCurrentHeartbeats(heartbeat)
						startStopListener.Reset()
						simulator.Tick(simulator.GracePeriod * 2) //after a long time
					})

					It("should not stop the other instance", func() {
						Expect(startStopListener.StopCount()).To(Equal(0))
					})
				})
			})

			Context("if only one instance is still running", func() {
				BeforeEach(func() {
					heartbeat = dea.HeartbeatWith(instance0.Heartbeat(), instance1.Heartbeat())
					simulator.SetCurrentHeartbeats(heartbeat)
					startStopListener.Reset()
					simulator.Tick(simulator.GracePeriod * 5) //after a long time
				})

				It("should not stop any instances", func() {
					Expect(startStopListener.StopCount()).To(Equal(0))
				})
			})
		})
	})
})
