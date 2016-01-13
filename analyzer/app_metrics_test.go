package analyzer_test

import (
	"errors"

	"github.com/cloudfoundry/dropsonde/metric_sender/fake"
	"github.com/cloudfoundry/dropsonde/metrics"
	. "github.com/cloudfoundry/hm9000/analyzer"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppMetrics", func() {
	var (
		sender *fake.FakeMetricSender
		apps   map[string]*models.App
		err    error
	)

	BeforeEach(func() {
		sender = fake.NewFakeMetricSender()
		metrics.Initialize(sender, nil)

		apps = map[string]*models.App{}
		err = nil
	})

	JustBeforeEach(func() {
		SendMetrics(apps, err)
	})

	Context("When it fails to fetch actual state data", func() {
		BeforeEach(func() {
			err = errors.New("failed to fetch actual state data")
		})

		It("should report metrics as -1", func() {
			Expect(sender.GetValue(NumberOfAppsWithAllInstancesReporting)).To(Equal(fake.Metric{-1, "Metric"}))
			Expect(sender.GetValue(NumberOfAppsWithMissingInstances)).To(Equal(fake.Metric{-1, "Metric"}))
			Expect(sender.GetValue(NumberOfUndesiredRunningApps)).To(Equal(fake.Metric{-1, "Metric"}))
			Expect(sender.GetValue(NumberOfRunningInstances)).To(Equal(fake.Metric{-1, "Metric"}))
			Expect(sender.GetValue(NumberOfMissingIndices)).To(Equal(fake.Metric{-1, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedInstances)).To(Equal(fake.Metric{-1, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedIndices)).To(Equal(fake.Metric{-1, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredApps)).To(Equal(fake.Metric{-1, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredInstances)).To(Equal(fake.Metric{-1, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredAppsPendingStaging)).To(Equal(fake.Metric{-1, "Metric"}))
		})
	})

	Context("When a desired app is pending staging", func() {
		BeforeEach(func() {
			fixture1 := appfixture.NewAppFixture()
			desired := fixture1.DesiredState(3)
			desired.PackageState = models.AppPackageStatePending

			apps = map[string]*models.App{
				"app1": models.NewApp(fixture1.AppGuid, fixture1.AppVersion,
					desired,
					fixture1.Heartbeat(1).InstanceHeartbeats,
					nil),
			}
		})

		It("should have the correct stats", func() {
			Expect(sender.GetValue(NumberOfAppsWithAllInstancesReporting)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfAppsWithMissingInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfUndesiredRunningApps)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfRunningInstances)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfMissingIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredApps)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredAppsPendingStaging)).To(Equal(fake.Metric{1, "Metric"}))
		})
	})

	Context("when a desired app has all instances running", func() {
		BeforeEach(func() {
			fixture1 := appfixture.NewAppFixture()
			apps = map[string]*models.App{
				"app1": models.NewApp(fixture1.AppGuid, fixture1.AppVersion,
					fixture1.DesiredState(3),
					fixture1.Heartbeat(3).InstanceHeartbeats,
					nil),
			}
		})

		It("should have the correct stats", func() {
			Expect(sender.GetValue(NumberOfAppsWithAllInstancesReporting)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfAppsWithMissingInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfUndesiredRunningApps)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfRunningInstances)).To(Equal(fake.Metric{3, "Metric"}))
			Expect(sender.GetValue(NumberOfMissingIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredApps)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredInstances)).To(Equal(fake.Metric{3, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredAppsPendingStaging)).To(Equal(fake.Metric{0, "Metric"}))
		})
	})

	Context("when a desired app has an instance starting and others running", func() {
		BeforeEach(func() {
			dea := appfixture.NewDeaFixture()
			fixture1 := dea.GetApp(0)
			startingHB := fixture1.InstanceAtIndex(1).Heartbeat()
			startingHB.State = models.InstanceStateStarting

			apps = map[string]*models.App{
				"app1": models.NewApp(fixture1.AppGuid, fixture1.AppVersion,
					fixture1.DesiredState(3),
					dea.HeartbeatWith(
						fixture1.InstanceAtIndex(0).Heartbeat(),
						startingHB,
						fixture1.InstanceAtIndex(2).Heartbeat(),
					).InstanceHeartbeats,
					nil),
			}
		})

		It("should have the correct stats", func() {
			Expect(sender.GetValue(NumberOfAppsWithAllInstancesReporting)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfAppsWithMissingInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfUndesiredRunningApps)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfRunningInstances)).To(Equal(fake.Metric{3, "Metric"}))
			Expect(sender.GetValue(NumberOfMissingIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredApps)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredInstances)).To(Equal(fake.Metric{3, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredAppsPendingStaging)).To(Equal(fake.Metric{0, "Metric"}))
		})
	})

	Context("when a desired app has crashed instances on some of the indices", func() {
		BeforeEach(func() {
			dea := appfixture.NewDeaFixture()
			fixture1 := dea.GetApp(0)

			apps = map[string]*models.App{
				"app1": models.NewApp(fixture1.AppGuid, fixture1.AppVersion,
					fixture1.DesiredState(3),
					dea.HeartbeatWith(
						fixture1.InstanceAtIndex(0).Heartbeat(),
						fixture1.InstanceAtIndex(1).Heartbeat(),
						fixture1.InstanceAtIndex(2).Heartbeat(),

						fixture1.CrashedInstanceHeartbeatAtIndex(1),
						fixture1.CrashedInstanceHeartbeatAtIndex(2),
						fixture1.CrashedInstanceHeartbeatAtIndex(2),
					).InstanceHeartbeats,
					nil),
			}
		})

		It("should have the correct stats", func() {
			Expect(sender.GetValue(NumberOfAppsWithAllInstancesReporting)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfAppsWithMissingInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfUndesiredRunningApps)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfRunningInstances)).To(Equal(fake.Metric{3, "Metric"}))
			Expect(sender.GetValue(NumberOfMissingIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedInstances)).To(Equal(fake.Metric{3, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredApps)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredInstances)).To(Equal(fake.Metric{3, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredAppsPendingStaging)).To(Equal(fake.Metric{0, "Metric"}))
		})
	})

	Context("when a desired app has at least one of the desired instances reporting as crashed", func() {
		BeforeEach(func() {
			dea := appfixture.NewDeaFixture()
			fixture1 := dea.GetApp(0)

			apps = map[string]*models.App{
				"app1": models.NewApp(fixture1.AppGuid, fixture1.AppVersion,
					fixture1.DesiredState(3),
					dea.HeartbeatWith(
						fixture1.InstanceAtIndex(0).Heartbeat(),
						fixture1.CrashedInstanceHeartbeatAtIndex(1),
						fixture1.CrashedInstanceHeartbeatAtIndex(1),
						fixture1.InstanceAtIndex(2).Heartbeat()).InstanceHeartbeats,
					nil),
			}
		})

		It("should have the correct stats", func() {
			Expect(sender.GetValue(NumberOfAppsWithAllInstancesReporting)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfAppsWithMissingInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfUndesiredRunningApps)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfRunningInstances)).To(Equal(fake.Metric{2, "Metric"}))
			Expect(sender.GetValue(NumberOfMissingIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedInstances)).To(Equal(fake.Metric{2, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedIndices)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredApps)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredInstances)).To(Equal(fake.Metric{3, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredAppsPendingStaging)).To(Equal(fake.Metric{0, "Metric"}))
		})
	})

	Context("when a desired app has at least one of the desired instances missing", func() {
		BeforeEach(func() {
			dea := appfixture.NewDeaFixture()
			fixture1 := dea.GetApp(0)

			apps = map[string]*models.App{
				"app1": models.NewApp(fixture1.AppGuid, fixture1.AppVersion,
					fixture1.DesiredState(3),
					dea.HeartbeatWith(
						fixture1.InstanceAtIndex(0).Heartbeat(),
						fixture1.InstanceAtIndex(2).Heartbeat()).InstanceHeartbeats,
					nil),
			}
		})

		It("should have the correct stats", func() {
			Expect(sender.GetValue(NumberOfAppsWithAllInstancesReporting)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfAppsWithMissingInstances)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfUndesiredRunningApps)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfRunningInstances)).To(Equal(fake.Metric{2, "Metric"}))
			Expect(sender.GetValue(NumberOfMissingIndices)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredApps)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredInstances)).To(Equal(fake.Metric{3, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredAppsPendingStaging)).To(Equal(fake.Metric{0, "Metric"}))
		})
	})

	Context("when a desired app has all of the desired instances missing", func() {
		BeforeEach(func() {
			dea := appfixture.NewDeaFixture()
			fixture1 := dea.GetApp(0)

			apps = map[string]*models.App{
				"app1": models.NewApp(fixture1.AppGuid, fixture1.AppVersion,
					fixture1.DesiredState(3),
					nil,
					nil),
			}
		})

		It("should have the correct stats", func() {
			Expect(sender.GetValue(NumberOfAppsWithAllInstancesReporting)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfAppsWithMissingInstances)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfUndesiredRunningApps)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfRunningInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfMissingIndices)).To(Equal(fake.Metric{3, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredApps)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredInstances)).To(Equal(fake.Metric{3, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredAppsPendingStaging)).To(Equal(fake.Metric{0, "Metric"}))
		})
	})

	Context("when there is an undesired app that is reporting as running", func() {
		BeforeEach(func() {
			dea := appfixture.NewDeaFixture()
			fixture1 := dea.GetApp(0)
			fixture2 := dea.GetApp(1)

			apps = map[string]*models.App{
				"app1": models.NewApp(fixture1.AppGuid, fixture1.AppVersion,
					models.DesiredAppState{},
					dea.HeartbeatWith(
						fixture1.InstanceAtIndex(0).Heartbeat(),
						fixture1.CrashedInstanceHeartbeatAtIndex(1),
						fixture1.InstanceAtIndex(2).Heartbeat(),
					).InstanceHeartbeats,
					nil),
				"app2": models.NewApp(fixture2.AppGuid, fixture2.AppVersion,
					models.DesiredAppState{},
					dea.HeartbeatWith(
						fixture2.InstanceAtIndex(0).Heartbeat(),
					).InstanceHeartbeats,
					nil),
			}
		})

		It("should have the correct stats", func() {
			Expect(sender.GetValue(NumberOfAppsWithAllInstancesReporting)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfAppsWithMissingInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfUndesiredRunningApps)).To(Equal(fake.Metric{2, "Metric"}))
			Expect(sender.GetValue(NumberOfRunningInstances)).To(Equal(fake.Metric{3, "Metric"}))
			Expect(sender.GetValue(NumberOfMissingIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedInstances)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedIndices)).To(Equal(fake.Metric{1, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredApps)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredAppsPendingStaging)).To(Equal(fake.Metric{0, "Metric"}))
		})
	})

	Context("when there is an undesired app that is reporting as crashed", func() {
		BeforeEach(func() {
			dea := appfixture.NewDeaFixture()
			fixture1 := dea.GetApp(0)

			apps = map[string]*models.App{
				"app1": models.NewApp(fixture1.AppGuid, fixture1.AppVersion,
					models.DesiredAppState{},
					dea.HeartbeatWith(
						fixture1.CrashedInstanceHeartbeatAtIndex(0),
						fixture1.CrashedInstanceHeartbeatAtIndex(1),
					).InstanceHeartbeats,
					nil),
			}
		})

		It("should have the correct stats", func() {
			Expect(sender.GetValue(NumberOfAppsWithAllInstancesReporting)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfAppsWithMissingInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfUndesiredRunningApps)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfRunningInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfMissingIndices)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedInstances)).To(Equal(fake.Metric{2, "Metric"}))
			Expect(sender.GetValue(NumberOfCrashedIndices)).To(Equal(fake.Metric{2, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredApps)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredInstances)).To(Equal(fake.Metric{0, "Metric"}))
			Expect(sender.GetValue(NumberOfDesiredAppsPendingStaging)).To(Equal(fake.Metric{0, "Metric"}))
		})
	})
})
