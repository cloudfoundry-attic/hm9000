package app_test

import (
    . "github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/cloudfoundry/hm9000/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

		"time"
)

var _ = Describe("App Model", func() {
	var app App

	BeforeEach(func() {
		app = NewApp()
	})

	It("makes a reasonable app", func() {
		Ω(app.AppGuid).ShouldNot(BeEmpty())
		Ω(app.AppVersion).ShouldNot(BeEmpty())
	})

	Describe("the desired state message", func() {
		It("generates one with sane defaults", func() {
			desired := app.DesiredState(12)

			Ω(desired.AppGuid).Should(Equal(app.AppGuid))
			Ω(desired.AppVersion).Should(Equal(app.AppVersion))
			Ω(desired.NumberOfInstances).Should(BeNumerically("==", 1))
			Ω(desired.Memory).Should(BeNumerically("==", 1024))
			Ω(desired.State).Should(Equal(AppStateStarted))
			Ω(desired.PackageState).Should(Equal(AppPackageStateStaged))
			Ω(desired.UpdatedAt).Should(Equal(time.Unix(12, 0)))
		})

		It("can generate an array of one desired state for convenience", func() {
			Ω(app.DesiredStateArr(12)).Should(Equal([]DesiredAppState{app.DesiredState(12)}))
		})
	})

	Describe("GetInstance", func() {
		It("creates and memoizes instance", func() {
			instance := app.GetInstance(0)

			Ω(instance.AppGuid).Should(Equal(app.AppGuid))
			Ω(instance.AppVersion).Should(Equal(app.AppVersion))
			Ω(instance.InstanceGuid).ShouldNot(BeEmpty())
			Ω(instance.InstanceIndex).Should(Equal(0))

			instanceAgain := app.GetInstance(0)
			Ω(instanceAgain).Should(Equal(instance))

			otherInstance := app.GetInstance(3)
			Ω(otherInstance.InstanceIndex).Should(Equal(3))
			Ω(otherInstance.InstanceGuid).ShouldNot(Equal(instance.InstanceGuid))
		})
	})

	Describe("Instance", func() {
		var instance Instance
		BeforeEach(func() {
			instance = app.GetInstance(0)
		})

		Describe("Heartbeat", func() {
			It("creates an instance heartbeat", func() {
				heartbeat := instance.Heartbeat(1)

				Ω(heartbeat.CCPartition).Should(Equal("default"))
				Ω(heartbeat.AppGuid).Should(Equal(app.AppGuid))
				Ω(heartbeat.AppVersion).Should(Equal(app.AppVersion))
				Ω(heartbeat.InstanceGuid).Should(Equal(instance.InstanceGuid))
				Ω(heartbeat.InstanceIndex).Should(Equal(instance.InstanceIndex))
				Ω(heartbeat.State).Should(Equal(InstanceStateRunning))
				Ω(heartbeat.StateTimestamp).Should(BeNumerically("==", 1))
			})
		})

		Describe("DropletExited", func() {
			It("returns droplet exited with the passed in reason", func() {
				exited := instance.DropletExited(DropletExitedReasonStopped, 12381)

				Ω(exited.CCPartition).Should(Equal("default"))
				Ω(exited.AppGuid).Should(Equal(app.AppGuid))
				Ω(exited.AppVersion).Should(Equal(app.AppVersion))
				Ω(exited.InstanceGuid).Should(Equal(instance.InstanceGuid))
				Ω(exited.InstanceIndex).Should(Equal(instance.InstanceIndex))
				Ω(exited.Reason).Should(Equal(DropletExitedReasonStopped))
				Ω(exited.ExitStatusCode).Should(Equal(0))
				Ω(exited.ExitDescription).Should(Equal("exited"))
				Ω(exited.CrashTimestamp).Should(BeZero())
			})

			Context("when the reason is crashed", func() {
				It("includes the crash timestamp", func() {
					exited := instance.DropletExited(DropletExitedReasonCrashed, 17)
					Ω(exited.Reason).Should(Equal(DropletExitedReasonCrashed))
					Ω(exited.ExitStatusCode).Should(Equal(1))
					Ω(exited.ExitDescription).Should(Equal("crashed"))
					Ω(exited.CrashTimestamp).Should(Equal(int64(17)))
				})
			})
		})
	})

	Describe("Heartbeat", func() {
		It("creates a heartbeat for the desired number of instances, using the correct instnace guids when available", func() {
			instance := app.GetInstance(0)
			heartbeat := app.Heartbeat(2, 1)

			Ω(heartbeat.DeaGuid).ShouldNot(BeEmpty())

			Ω(heartbeat.InstanceHeartbeats).Should(HaveLen(2))
			Ω(heartbeat.InstanceHeartbeats[0]).Should(Equal(instance.Heartbeat(1)))
			Ω(heartbeat.InstanceHeartbeats[1]).Should(Equal(app.GetInstance(1).Heartbeat(1)))

			Ω(app.Heartbeat(2, 1)).Should(Equal(heartbeat))
		})
	})

	Describe("Droplet Updated", func() {
		It("creates a droplet.updated message with the correct guid", func() {
			droplet_updated := app.DropletUpdated()

			Ω(droplet_updated.AppGuid).Should(Equal(app.AppGuid))
		})
	})
})
