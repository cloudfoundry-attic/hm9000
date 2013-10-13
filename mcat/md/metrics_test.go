package md_test

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"net/http"
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
		Ω(err).ShouldNot(HaveOccured())
	})

	AfterEach(func() {
		cliRunner.StopMetricsServer()
	})

	It("should register with the collector", func(done Done) {
		cliRunner.StartMetricsServer(simulator.currentTimestamp)
		natsRunner.MessageBus.Request("vcap.component.discover", []byte{}, func(msg []byte) {
			Ω(string(msg)).Should(ContainSubstring("%s:%d", ip, metricsServerPort))
			Ω(string(msg)).Should(ContainSubstring(`"bob","password"`))
			close(done)
		})
	})

	Context("when the store is fresh", func() {
		BeforeEach(func() {
			simulator.Tick(simulator.TicksToAttainFreshness)
			cliRunner.StartMetricsServer(simulator.currentTimestamp)
		})

		It("should return the metrics", func() {
			resp, err := http.Get(fmt.Sprintf("http://bob:password@%s:%d/varz", ip, metricsServerPort))
			Ω(err).ShouldNot(HaveOccured())

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			bodyAsString := string(body)
			Ω(err).ShouldNot(HaveOccured())
			Ω(bodyAsString).Should(ContainSubstring(`"name":"NumberOfUndesiredRunningApps","value":0`))
			Ω(bodyAsString).Should(ContainSubstring(`"name":"NumberOfAppsWithMissingInstances","value":1`))
			Ω(bodyAsString).Should(ContainSubstring(`"name":"HM9000"`))
		})
	})

	Context("when the store is not fresh", func() {
		BeforeEach(func() {
			simulator.Tick(simulator.TicksToAttainFreshness - 1)
			cliRunner.StartMetricsServer(simulator.currentTimestamp)
		})

		It("should return -1 for all metrics", func() {
			resp, err := http.Get(fmt.Sprintf("http://bob:password@%s:%d/varz", ip, metricsServerPort))
			Ω(err).ShouldNot(HaveOccured())

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			bodyAsString := string(body)
			Ω(err).ShouldNot(HaveOccured())
			Ω(bodyAsString).Should(ContainSubstring(`"name":"NumberOfUndesiredRunningApps","value":-1`))
			Ω(bodyAsString).Should(ContainSubstring(`"name":"NumberOfAppsWithMissingInstances","value":-1`))
			Ω(bodyAsString).Should(ContainSubstring(`"name":"HM9000"`))
		})
	})
})
