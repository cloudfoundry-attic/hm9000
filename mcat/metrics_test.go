package mcat_test

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/nats-io/nats"
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
		Ω(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		cliRunner.StopMetricsServer()
	})

	It("should register with the collector", func(done Done) {
		cliRunner.StartMetricsServer(simulator.currentTimestamp)
		guid := models.Guid()

		coordinator.MessageBus.Subscribe(guid, func(message *nats.Msg) {
			Ω(string(message.Data)).Should(ContainSubstring("%s:%d", ip, coordinator.MetricsServerPort))
			Ω(string(message.Data)).Should(ContainSubstring(`"bob","password"`))
			close(done)
		})

		coordinator.MessageBus.PublishRequest("vcap.component.discover", guid, []byte(""))
	})

	Context("when there is a desired app that failed to stage", func() {
		BeforeEach(func() {
			desiredState := a.DesiredState(2)
			desiredState.PackageState = models.AppPackageStateFailed
			simulator.SetDesiredState(desiredState)
			simulator.SetCurrentHeartbeats(a.Heartbeat(1))

			simulator.Tick(simulator.TicksToAttainFreshness)
			cliRunner.StartMetricsServer(simulator.currentTimestamp)
		})

		It("should not count as an app with missing instances", func() {
			resp, err := http.Get(fmt.Sprintf("http://bob:password@%s:%d/varz", ip, coordinator.MetricsServerPort))
			Ω(err).ShouldNot(HaveOccurred())

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			bodyAsString := string(body)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(bodyAsString).Should(ContainSubstring(`"name":"NumberOfAppsWithMissingInstances","value":0`))
		})
	})

	Context("when the store is fresh", func() {
		BeforeEach(func() {
			simulator.Tick(simulator.TicksToAttainFreshness)
			simulator.Tick(simulator.GracePeriod)
			cliRunner.StartMetricsServer(simulator.currentTimestamp)
		})

		It("should return the metrics", func() {
			resp, err := http.Get(fmt.Sprintf("http://bob:password@%s:%d/varz", ip, coordinator.MetricsServerPort))
			Ω(err).ShouldNot(HaveOccurred())

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			bodyAsString := string(body)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(bodyAsString).Should(ContainSubstring(`"name":"NumberOfUndesiredRunningApps","value":0`))
			Ω(bodyAsString).Should(ContainSubstring(`"name":"NumberOfAppsWithMissingInstances","value":1`))
			Ω(bodyAsString).Should(ContainSubstring(`"name":"StartMissing","value":1`))
			Ω(bodyAsString).Should(ContainSubstring(`"name":"HM9000"`))
		})
	})

	Context("when the store is not fresh", func() {
		BeforeEach(func() {
			simulator.Tick(simulator.TicksToAttainFreshness - 1)
			cliRunner.StartMetricsServer(simulator.currentTimestamp)
		})

		It("should return -1 for all metrics", func() {
			resp, err := http.Get(fmt.Sprintf("http://bob:password@%s:%d/varz", ip, coordinator.MetricsServerPort))
			Ω(err).ShouldNot(HaveOccurred())

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			bodyAsString := string(body)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(bodyAsString).Should(ContainSubstring(`"name":"NumberOfUndesiredRunningApps","value":-1`))
			Ω(bodyAsString).Should(ContainSubstring(`"name":"NumberOfAppsWithMissingInstances","value":-1`))
			Ω(bodyAsString).Should(ContainSubstring(`"name":"HM9000"`))
		})
	})
})
