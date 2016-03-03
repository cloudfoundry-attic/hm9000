package mcat_test

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Simple Cases Test", func() {
	var app1, app2 appfixture.AppFixture

	BeforeEach(func() {
		app1 = appfixture.NewAppFixture()
		app2 = appfixture.NewAppFixture()
	})

	Context("when all running instances are desired", func() {
		BeforeEach(func() {
			simulator.SetCurrentHeartbeats(app1.Heartbeat(1), app2.Heartbeat(1))
			simulator.SetDesiredState(app1.DesiredState(1), app2.DesiredState(1))
			simulator.Tick(simulator.TicksToAttainFreshness)
			simulator.Tick(1)
		})

		It("should not send any messages", func() {
			Expect(startStopListener.StartCount()).To(Equal(0))
			Expect(startStopListener.StopCount()).To(Equal(0))
		})
	})

	Context("when a desired app is pending staging", func() {
		Context("and it has a running instance", func() {
			BeforeEach(func() {
				desired := app1.DesiredState(1)
				desired.PackageState = models.AppPackageStatePending
				simulator.SetDesiredState(desired)
				simulator.SetCurrentHeartbeats(app1.Heartbeat(1))
				simulator.Tick(simulator.TicksToAttainFreshness)
				simulator.Tick(1)
			})

			It("should not try to stop that instance", func() {
				Expect(startStopListener.StartCount()).To(Equal(0))
				Expect(startStopListener.StopCount()).To(Equal(0))
			})
		})
	})

	Context("when there is a missing instance", func() {
		BeforeEach(func() {
			simulator.SetCurrentHeartbeats(app1.Heartbeat(1), app2.Heartbeat(1))
			simulator.SetDesiredState(app1.DesiredState(1), app2.DesiredState(2))
			simulator.Tick(simulator.TicksToAttainFreshness) //this tick will schedule a start

			// no message is sent during the start send message delay
			simulator.Tick(1)
			Expect(startStopListener.StartCount()).To(Equal(0))

			simulator.Tick(1)
			Expect(startStopListener.StartCount()).To(Equal(0))
		})

		Context("when the instance recovers on its own", func() {
			BeforeEach(func() {
				simulator.SetCurrentHeartbeats(app1.Heartbeat(1), app2.Heartbeat(2))
				simulator.Tick(1)
			})

			It("should not send a start message", func() {
				Expect(startStopListener.StartCount()).To(Equal(0))
			})
		})

		Context("when the instance is no longer desired", func() {
			BeforeEach(func() {
				simulator.SetDesiredState(app1.DesiredState(1), app2.DesiredState(1))
				simulator.Tick(1)
			})

			It("should not send a start message", func() {
				Expect(startStopListener.StartCount()).To(Equal(0))
			})
		})

		Context("when the instance does not recover on its own", func() {
			BeforeEach(func() {
				simulator.Tick(1)
			})

			It("should send a start message, after a delay, for the missing instance", func() {
				Expect(startStopListener.StartCount()).To(Equal(1))

				start := startStopListener.Start(0)
				Expect(start.AppGuid).To(Equal(app2.AppGuid))
				Expect(start.AppVersion).To(Equal(app2.AppVersion))
				Expect(start.InstanceIndex).To(Equal(1))
			})
		})
	})

	Context("when there is an undesired instance running", func() {
		BeforeEach(func() {
			simulator.SetDesiredState(app2.DesiredState(1))
			simulator.SetCurrentHeartbeats(app2.Heartbeat(2))
			simulator.Tick(simulator.TicksToAttainFreshness)
		})

		Context("when the instance becomes desired", func() {
			BeforeEach(func() {
				simulator.SetDesiredState(app2.DesiredState(2))
				startStopListener.Reset()
				simulator.Tick(1)
			})

			It("should not send a stop message", func() {
				Expect(startStopListener.StopCount()).To(Equal(0))
			})
		})

		Context("when the app is still running", func() {
			BeforeEach(func() {
				simulator.Tick(1)
			})

			It("should send a stop message, immediately, for the missing instance", func() {
				Expect(startStopListener.StopCount()).To(Equal(1))

				stop := startStopListener.Stop(0)
				Expect(stop.AppGuid).To(Equal(app2.AppGuid))
				Expect(stop.AppVersion).To(Equal(app2.AppVersion))
				Expect(stop.InstanceGuid).To(Equal(app2.InstanceAtIndex(1).InstanceGuid))
				Expect(stop.InstanceIndex).To(Equal(1))
				Expect(stop.IsDuplicate).To(BeFalse())
			})
		})
	})
})
