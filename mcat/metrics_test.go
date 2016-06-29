package mcat_test

import (
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/localip"
)

var _ = Describe("Serving Metrics", func() {
	var (
		a  appfixture.AppFixture
		ip string
	)

	BeforeEach(func() {
		a = appfixture.NewAppFixture()

		simulator.SetDesiredState(a.DesiredState(2))
		simulator.SetCurrentHeartbeats(a.Heartbeat(1))

		var err error
		ip, err = localip.LocalIP()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when there is a desired app that failed to stage", func() {
		BeforeEach(func() {
			desiredState := a.DesiredState(2)
			desiredState.PackageState = models.AppPackageStateFailed
			simulator.SetDesiredState(desiredState)
			simulator.SetCurrentHeartbeats(a.Heartbeat(1))

			simulator.Tick(simulator.TicksToAttainFreshness, false)
		})

		It("should not count as an app with missing instances", func() {
			Eventually(func() bool {
				return metronAgent.MatchEvent("analyzer", events.Envelope_ValueMetric, "NumberOfAppsWithMissingInstances", 0.0)
			}).Should(BeTrue())

			Consistently(func() bool {
				return metronAgent.MatchEvent("analyzer", events.Envelope_ValueMetric, "NumberOfAppsWithMissingInstances", 0.0)
			}).Should(BeTrue())
		})
	})

	Context("when the store is fresh", func() {
		BeforeEach(func() {
			simulator.Tick(simulator.TicksToAttainFreshness, false)
			simulator.Tick(simulator.GracePeriod, false)
		})

		It("should return the metrics", func() {
			Eventually(func() bool {
				return metronAgent.MatchEvent("analyzer", events.Envelope_ValueMetric, "NumberOfAppsWithMissingInstances", 1.0)
			}).Should(BeTrue())
			Eventually(func() bool {
				return metronAgent.MatchEvent("analyzer", events.Envelope_ValueMetric, "NumberOfUndesiredRunningApps", 0.0)
			}).Should(BeTrue())
			Eventually(func() bool {
				return metronAgent.MatchEvent("analyzer", events.Envelope_CounterEvent, metricsaccountant.StartMissing, 1.0)
			}).Should(BeTrue())
		})
	})

	Context("when the store is not fresh", func() {
		BeforeEach(func() {
			simulator.Tick(simulator.TicksToAttainFreshness - 1, false)
		})

		It("should return -1 for all metrics", func() {
			Eventually(func() bool {
				return metronAgent.MatchEvent("analyzer", events.Envelope_ValueMetric, "NumberOfAppsWithMissingInstances", -1.0)
			}).Should(BeTrue())
			Eventually(func() bool {
				return metronAgent.MatchEvent("analyzer", events.Envelope_ValueMetric, "NumberOfUndesiredRunningApps", -1.0)
			}).Should(BeTrue())
		})
	})
})
