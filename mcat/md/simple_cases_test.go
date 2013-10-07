package md_test

import (
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Simple Cases Test", func() {
	var app1, app2 app.App
	var timestamp int

	Context("when all running instances are desired", func() {
		BeforeEach(func() {
			app1 = app.NewApp()
			app2 = app.NewApp()

			stateServer.SetDesiredState([]models.DesiredAppState{
				app1.DesiredState(0),
				app2.DesiredState(0),
			})

			timestamp = 100

			for i := 0; i < 3; i++ {
				sendHeartbeats(timestamp, app1.Heartbeat(1, 0), app2.Heartbeat(1, 0))
				timestamp += 10
			}

			cliRunner.Run("fetch_desired", timestamp)
			cliRunner.Run("analyze", timestamp)
			cliRunner.Run("send", timestamp)
		})

		It("should send a start message for the missing instance", func() {
			Ω(startStopListener.Starts).Should(BeEmpty())
			Ω(startStopListener.Stops).Should(BeEmpty())
		})
	})

	Context("when there is a missing instance", func() {
		BeforeEach(func() {
			timestamp = 100

			app1 = app.NewApp()
			app2 = app.NewApp()

			desired := app2.DesiredState(0)
			desired.NumberOfInstances = 2

			stateServer.SetDesiredState([]models.DesiredAppState{
				app1.DesiredState(0),
				desired,
			})

			for i := 0; i < 3; i++ {
				sendHeartbeats(timestamp, app1.Heartbeat(1, 0), app2.Heartbeat(1, 0))
				timestamp += 10
			}

			cliRunner.Run("fetch_desired", timestamp)

			cliRunner.Run("analyze", timestamp)

			sendHeartbeats(timestamp, app1.Heartbeat(1, 0), app2.Heartbeat(1, 0))
			timestamp += 10
			cliRunner.Run("send", timestamp)
			Ω(startStopListener.Starts).Should(BeEmpty())

			sendHeartbeats(timestamp, app1.Heartbeat(1, 0), app2.Heartbeat(1, 0))
			timestamp += 10
			cliRunner.Run("send", timestamp)
			Ω(startStopListener.Starts).Should(BeEmpty())

			sendHeartbeats(timestamp, app1.Heartbeat(1, 0), app2.Heartbeat(1, 0))
			timestamp += 10
			cliRunner.Run("fetch_desired", timestamp)
		})

		Context("when the app recovers on its own", func() {
			BeforeEach(func() {
				sendHeartbeats(timestamp, app1.Heartbeat(1, 0), app2.Heartbeat(2, 0))
				cliRunner.Run("send", timestamp)
			})

			It("should not send a start message", func() {
				Ω(startStopListener.Starts).Should(HaveLen(0))
			})
		})

		Context("when the app is no longer desired", func() {
			BeforeEach(func() {
				stateServer.SetDesiredState([]models.DesiredAppState{
					app1.DesiredState(0),
					app2.DesiredState(0),
				})
				cliRunner.Run("fetch_desired", timestamp)
				cliRunner.Run("send", timestamp)
			})

			It("should not send a start message", func() {
				Ω(startStopListener.Starts).Should(HaveLen(0))
			})
		})

		Context("when the app does not recover on its own", func() {
			BeforeEach(func() {
				cliRunner.Run("send", timestamp)
			})

			It("should send a start message for the missing instance", func() {
				Ω(startStopListener.Starts).Should(HaveLen(1))

				start := startStopListener.Starts[0]
				Ω(start.AppGuid).Should(Equal(app2.AppGuid))
				Ω(start.AppVersion).Should(Equal(app2.AppVersion))
				Ω(start.InstanceIndex).Should(Equal(1))
			})
		})
	})

	Context("when there is an undesired instance running", func() {
		BeforeEach(func() {
			app1 = app.NewApp()
			app2 = app.NewApp()

			stateServer.SetDesiredState([]models.DesiredAppState{
				app1.DesiredState(0),
				app2.DesiredState(0),
			})

			timestamp = 100

			for i := 0; i < 3; i++ {
				sendHeartbeats(timestamp, app1.Heartbeat(1, 0), app2.Heartbeat(2, 0))
				timestamp += 10
			}

			cliRunner.Run("fetch_desired", timestamp)
			cliRunner.Run("analyze", timestamp)
		})

		Context("when the instance is no longer running", func() {
			BeforeEach(func() {
				expireHeartbeat(app2.InstanceAtIndex(1).Heartbeat(0))
				cliRunner.Run("send", timestamp)
			})

			It("should not send a stop message", func() {
				Ω(startStopListener.Stops).Should(HaveLen(0))
			})
		})

		Context("when the instance becomes desired", func() {
			BeforeEach(func() {
				desired := app2.DesiredState(0)
				desired.NumberOfInstances = 2

				stateServer.SetDesiredState([]models.DesiredAppState{
					app1.DesiredState(0),
					desired,
				})
				cliRunner.Run("fetch_desired", timestamp)
				cliRunner.Run("send", timestamp)
			})

			It("should not send a stop message", func() {
				Ω(startStopListener.Stops).Should(HaveLen(0))
			})
		})

		Context("when the app is still running", func() {
			BeforeEach(func() {
				cliRunner.Run("send", timestamp)
			})

			It("should send a stop message for the missing instance", func() {
				Ω(startStopListener.Stops).Should(HaveLen(1))

				stop := startStopListener.Stops[0]
				Ω(stop.AppGuid).Should(Equal(app2.AppGuid))
				Ω(stop.AppVersion).Should(Equal(app2.AppVersion))
				Ω(stop.InstanceGuid).Should(Equal(app2.InstanceAtIndex(1).InstanceGuid))
				Ω(stop.InstanceIndex).Should(Equal(1))
				Ω(stop.IsDuplicate).Should(BeFalse())
			})
		})
	})
})
