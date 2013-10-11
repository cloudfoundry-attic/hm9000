package md_test

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/localip"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"net/http"
)

var _ = Describe("Simple Cases Test", func() {
	var (
		a  app.App
		ip string
	)

	BeforeEach(func() {
		a = app.NewApp()

		simulator.SetDesiredState(a.DesiredState(2))
		simulator.SetCurrentHeartbeats(a.Heartbeat(1))

		var err error
		ip, err = localip.LocalIP()
		Ω(err).ShouldNot(HaveOccured())
	})

	AfterEach(func() {
		cliRunner.StopMetricsServer()
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
		})
	})
})
