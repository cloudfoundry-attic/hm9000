package mcat_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

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
			serverAddr = fmt.Sprintf("https://%s:%d", cliRunner.config.HttpHeartbeatServerAddress, cliRunner.config.HttpHeartbeatPort)
			routes := rata.Routes{{Method: "POST", Name: "dea_heartbeat_handler", Path: "/dea/heartbeat"}}

			requestGenerator = rata.NewRequestGenerator(serverAddr, routes)

			certFile, err := filepath.Abs("../testhelpers/fake_certs/hm9000_client.crt")
			Expect(err).NotTo(HaveOccurred())

			keyFile, err := filepath.Abs("../testhelpers/fake_certs/hm9000_client.key")
			Expect(err).NotTo(HaveOccurred())

			cert, err := tls.LoadX509KeyPair(certFile, keyFile)
			Expect(err).NotTo(HaveOccurred())

			caCertFile, err := filepath.Abs("../testhelpers/fake_certs/hm9000_ca.crt")
			Expect(err).NotTo(HaveOccurred())

			caCert, err := ioutil.ReadFile(caCertFile)
			Expect(err).NotTo(HaveOccurred())

			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)

			tlsConfig := &tls.Config{
				Certificates: []tls.Certificate{cert},
				RootCAs:      caCertPool,
			}

			tlsConfig.BuildNameToCertificate()

			tlsTransport := &http.Transport{
				TLSClientConfig: tlsConfig,
			}

			httpClient = &http.Client{
				Transport: tlsTransport,
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
