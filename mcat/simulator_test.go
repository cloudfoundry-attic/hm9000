package mcat_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/cloudfoundry/yagnats"
	. "github.com/onsi/gomega"
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

func (s *Simulator) Tick(numTicks int) {
	timeBetweenTicks := int(s.conf.HeartbeatPeriod)

	for i := 0; i < numTicks; i++ {
		s.currentTimestamp += timeBetweenTicks
		s.storeRunner.FastForwardTime(timeBetweenTicks)
		s.sendHeartbeats()
		s.cliRunner.Run("analyze", s.currentTimestamp)
	}
}

func (s *Simulator) sendHeartbeats() {
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

func (s *Simulator) SetDesiredState(desiredStates ...models.DesiredAppState) {
	s.desiredStateServer.SetDesiredState(desiredStates)
}

func (s *Simulator) SetCurrentHeartbeats(heartbeats ...*models.Heartbeat) {
	s.currentHeartbeats = heartbeats
}
