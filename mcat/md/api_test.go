package md_test

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"net/http"
)

var _ = Describe("Serving API", func() {
	var (
		a appfixture.AppFixture
	)

	BeforeEach(func() {
		a = appfixture.NewAppFixture()

		simulator.SetDesiredState(a.DesiredState(2))
		simulator.SetCurrentHeartbeats(a.Heartbeat(1))
	})

	AfterEach(func() {
		cliRunner.StopAPIServer()
	})

	Context("when the store is fresh", func() {
		BeforeEach(func() {
			simulator.Tick(simulator.TicksToAttainFreshness)
			cliRunner.StartAPIServer(simulator.currentTimestamp)
		})

		It("should return the app", func() {
			resp, err := http.Get(fmt.Sprintf("http://%s:%s@localhost:%d/app?app-guid=%s&app-version=%s", conf.APIServerUser, conf.APIServerPassword, apiServerPort, a.AppGuid, a.AppVersion))
			Ω(err).ShouldNot(HaveOccured())

			Ω(resp.StatusCode).Should(Equal(http.StatusOK))

			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			bodyAsString := string(body)
			Ω(err).ShouldNot(HaveOccured())

			Ω(bodyAsString).Should(ContainSubstring(`"droplet":"%s"`, a.AppGuid))
			Ω(bodyAsString).Should(ContainSubstring(`"instances":2`))
			Ω(bodyAsString).Should(ContainSubstring(`"instance":"%s"`, a.InstanceAtIndex(0).InstanceGuid))
		})
	})

	Context("when the store is not fresh", func() {
		BeforeEach(func() {
			simulator.Tick(simulator.TicksToAttainFreshness - 1)
			cliRunner.StartAPIServer(simulator.currentTimestamp)
		})

		It("should return -1 for all metrics", func() {
			resp, err := http.Get(fmt.Sprintf("http://%s:%s@localhost:%d/app?app-guid=%s&app-version=%s", conf.APIServerUser, conf.APIServerPassword, apiServerPort, a.AppGuid, a.AppVersion))
			Ω(err).ShouldNot(HaveOccured())

			Ω(resp.StatusCode).Should(Equal(http.StatusNotFound))

			defer resp.Body.Close()
		})
	})
})
