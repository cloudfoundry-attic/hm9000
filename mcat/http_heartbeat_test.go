package mcat_test

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/sonde-go/events"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"
)

var _ = Describe("Serving HTTP Heartbeat", func() {
	var (
		dea              appfixture.DeaFixture
		requestGenerator *rata.RequestGenerator
		httpClient       *http.Client
		serverAddr       string
	)

	Describe("POST /dea/heartbeat", func() {

		BeforeEach(func() {
			serverAddr = fmt.Sprintf("http://%s:%d", cliRunner.config.HttpHeartbeatServerAddress, cliRunner.config.HttpHeartbeatPort)
			routes := rata.Routes{{Method: "POST", Name: "dea_heartbeat_handler", Path: "/dea/heartbeat"}}

			requestGenerator = rata.NewRequestGenerator(serverAddr, routes)

			httpClient = &http.Client{
				Transport: &http.Transport{},
			}

			dea = appfixture.NewDeaFixture()

			cliRunner.StartListener(simulator.currentTimestamp)
		})

		AfterEach(func() {
			cliRunner.StopListener()
		})

		Context("when an http heartbeat is sent", func() {
			It("is accepted by the http heartbeat handler", func() {
				sendHeartbeat, err := requestGenerator.CreateRequest(
					"dea_heartbeat_handler",
					nil,
					bytes.NewBuffer(dea.Heartbeat(1).ToJSON()),
				)

				Expect(err).NotTo(HaveOccurred())

				response, err := httpClient.Do(sendHeartbeat)
				Expect(err).NotTo(HaveOccurred())
				defer response.Body.Close()

				Expect(response.StatusCode).To(Equal(http.StatusAccepted))

				metronAgent.Reset()
				Eventually(func() bool {
					return metronAgent.MatchEvent("listener", events.Envelope_ValueMetric, "SavedHeartbeats", 1)
				}, 5.0, 0.05).Should(BeTrue())
			})
		})
	})
})
