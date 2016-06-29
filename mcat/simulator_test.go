package mcat_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/cloudfoundry/yagnats"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"
)

type NatsConnections struct {
	NumConnections int              `json:"num_connections"`
	Connections    []NatsConnection `json:"connections"`
}

type NatsConnection struct {
	Subscriptions     int      `json:"subscriptions"`
	SubscriptionsList []string `json:"subscriptions_list"`
}

type Simulator struct {
	conf                   *config.Config
	storeRunner            *etcdstorerunner.ETCDClusterRunner
	store                  store.Store
	desiredStateServer     *desiredstateserver.DesiredStateServer
	currentHeartbeats      []*models.Heartbeat
	currentTimestamp       int
	cliRunner              *CLIRunner
	TicksToAttainFreshness int
	TicksToExpireHeartbeat int
	GracePeriod            int
	messageBus             yagnats.NATSConn
	natsMonitoringPort     int
}

func NewSimulator(conf *config.Config, storeRunner *etcdstorerunner.ETCDClusterRunner, store store.Store, desiredStateServer *desiredstateserver.DesiredStateServer, cliRunner *CLIRunner, messageBus yagnats.NATSConn, natsMonitoringPort int) *Simulator {
	desiredStateServer.Reset()

	return &Simulator{
		currentTimestamp:       100,
		conf:                   conf,
		storeRunner:            storeRunner,
		store:                  store,
		desiredStateServer:     desiredStateServer,
		cliRunner:              cliRunner,
		TicksToAttainFreshness: int(conf.ActualFreshnessTTLInHeartbeats) + 1,
		TicksToExpireHeartbeat: int(conf.HeartbeatTTLInHeartbeats),
		GracePeriod:            int(conf.GracePeriodInHeartbeats),
		messageBus:             messageBus,
		natsMonitoringPort:     natsMonitoringPort,
	}
}

func (s *Simulator) Tick(numTicks int, useHttp bool) {
	timeBetweenTicks := int(s.conf.HeartbeatPeriod)

	for i := 0; i < numTicks; i++ {
		s.currentTimestamp += timeBetweenTicks
		s.storeRunner.FastForwardTime(timeBetweenTicks)
		s.sendHeartbeats(useHttp)
		s.cliRunner.Run("analyze", s.currentTimestamp)
	}
}

func (s *Simulator) sendHeartbeats(useHttp bool) {
	if useHttp {
		s.sendHeartbeatsHttp()
	} else {
		s.sendHeartbeatsNats()
	}
}

func (s *Simulator) sendHeartbeatsNats() {
	s.cliRunner.StartListener(s.currentTimestamp)
	metronAgent.Reset()

	Eventually(func() bool {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/connz?subs=1", s.natsMonitoringPort))
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())

		var natsConns NatsConnections
		err = json.Unmarshal(body, &natsConns)
		Expect(err).ToNot(HaveOccurred())

		for _, conn := range natsConns.Connections {
			for _, subscription := range conn.SubscriptionsList {
				if subscription == "dea.heartbeat" {
					return true
				}
			}
		}

		return false
	}, IterationTimeout).Should(BeTrue())

	for _, heartbeat := range s.currentHeartbeats {
		s.messageBus.Publish("dea.heartbeat", heartbeat.ToJSON())
	}

	nHeartbeats := len(s.currentHeartbeats)
	Eventually(func() bool {
		return metronAgent.GetMatchingEventTotal("listener", events.Envelope_ValueMetric, metricsaccountant.SavedHeartbeats) == float64(nHeartbeats)
	}, IterationTimeout, 1.05).Should(BeTrue())

	s.cliRunner.StopListener()
}

func (s *Simulator) sendHeartbeatsHttp() {
	metronAgent.Reset()

	serverAddr := fmt.Sprintf("https://%s:%d", cliRunner.config.HttpHeartbeatServerAddress, cliRunner.config.HttpHeartbeatPort)
	routes := rata.Routes{{Method: "POST", Name: "dea_heartbeat_handler", Path: "/dea/heartbeat"}}
	requestGenerator := rata.NewRequestGenerator(serverAddr, routes)

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

	httpClient := &http.Client{
		Transport: tlsTransport,
	}

	cliRunner.StartListener(s.currentTimestamp)
	for _, heartbeat := range s.currentHeartbeats {
		sendHeartbeat, err := requestGenerator.CreateRequest(
			"dea_heartbeat_handler",
			nil,
			bytes.NewBuffer(heartbeat.ToJSON()),
		)

		Expect(err).NotTo(HaveOccurred())

		response, err := httpClient.Do(sendHeartbeat)
		Expect(err).NotTo(HaveOccurred())
		defer response.Body.Close()

		Expect(response.StatusCode).To(Equal(http.StatusAccepted))
	}

	nHeartbeats := len(s.currentHeartbeats)
	Eventually(func() bool {
		return metronAgent.GetMatchingEventTotal("listener", events.Envelope_ValueMetric, metricsaccountant.SavedHeartbeats) == float64(nHeartbeats)
	}, IterationTimeout, 1.05).Should(BeTrue())

	cliRunner.StopListener()
}

func (s *Simulator) SetDesiredState(desiredStates ...models.DesiredAppState) {
	s.desiredStateServer.SetDesiredState(desiredStates)
}

func (s *Simulator) SetCurrentHeartbeats(heartbeats ...*models.Heartbeat) {
	s.currentHeartbeats = heartbeats
}
