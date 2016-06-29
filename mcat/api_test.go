package mcat_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry/hm9000/apiserver"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"
)

var _ = Describe("Serving API", func() {
	var (
		a                appfixture.AppFixture
		validRequest     string
		requestGenerator *rata.RequestGenerator
		httpClient       *http.Client
		apiServerAddr    string
		username         string
		password         string
	)

	Describe("POST /bulk_app_state", func() {
		var b appfixture.AppFixture

		BeforeEach(func() {
			apiServerAddr = fmt.Sprintf("http://%s:%d", cliRunner.config.APIServerAddress, cliRunner.config.APIServerPort)
			requestGenerator = rata.NewRequestGenerator(apiServerAddr, apiserver.Routes)
			httpClient = &http.Client{
				Transport: &http.Transport{},
			}

			username = cliRunner.config.APIServerUsername
			password = cliRunner.config.APIServerPassword

			a = appfixture.NewAppFixture()
			b = appfixture.NewAppFixture()
			validRequest = fmt.Sprintf(`[{"droplet":"%s","version":"%s"}, {"droplet":"%s","version":"%s"}]`, a.AppGuid, a.AppVersion, b.AppGuid, b.AppVersion)

			simulator.SetDesiredState(a.DesiredState(2), b.DesiredState(3))
			simulator.SetCurrentHeartbeats(a.Heartbeat(1), b.Heartbeat(1))
		})

		AfterEach(func() {
			cliRunner.StopAPIServer()
		})

		Context("when the store is fresh", func() {
			BeforeEach(func() {
				simulator.Tick(simulator.TicksToAttainFreshness, false)
				cliRunner.StartAPIServer(simulator.currentTimestamp)
			})

			It("returns the apps", func() {
				getAppstatus, err := requestGenerator.CreateRequest(
					"bulk_app_state",
					nil,
					bytes.NewBufferString(validRequest),
				)
				Expect(err).NotTo(HaveOccurred())

				getAppstatus.SetBasicAuth(username, password)
				response, err := httpClient.Do(getAppstatus)
				Expect(err).NotTo(HaveOccurred())
				defer response.Body.Close()

				Expect(response.StatusCode).To(Equal(http.StatusOK))

				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				bodyString := string(bodyBytes)

				Expect(bodyString).To(ContainSubstring(`"droplet":"%s"`, a.AppGuid))
				Expect(bodyString).To(ContainSubstring(`"droplet":"%s"`, b.AppGuid))
			})
		})

		Context("when the store is not fresh", func() {
			BeforeEach(func() {
				simulator.Tick(simulator.TicksToAttainFreshness - 1, false)
				cliRunner.StartAPIServer(simulator.currentTimestamp)
			})

			It("returns -1 for all metrics", func() {
				getAppstatus, err := requestGenerator.CreateRequest(
					"bulk_app_state",
					nil,
					bytes.NewBufferString(validRequest),
				)
				Expect(err).NotTo(HaveOccurred())

				getAppstatus.SetBasicAuth(username, password)
				response, err := httpClient.Do(getAppstatus)
				Expect(err).NotTo(HaveOccurred())
				defer response.Body.Close()

				Expect(response.StatusCode).To(Equal(http.StatusOK))

				bodyBytes, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				bodyString := string(bodyBytes)

				Expect(bodyString).To(BeEquivalentTo(`{}`))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				simulator.Tick(simulator.TicksToAttainFreshness, false)
				cliRunner.StartAPIServer(simulator.currentTimestamp)
			})

			It("returns a 401 Unauthorized", func() {
				getAppstatus, err := requestGenerator.CreateRequest(
					"bulk_app_state",
					nil,
					bytes.NewBufferString(validRequest),
				)
				Expect(err).NotTo(HaveOccurred())

				response, err := httpClient.Do(getAppstatus)
				Expect(err).NotTo(HaveOccurred())
				defer response.Body.Close()

				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
