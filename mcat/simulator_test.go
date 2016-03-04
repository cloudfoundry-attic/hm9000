package mcat_test

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/cloudfoundry/yagnats"
	. "github.com/onsi/gomega"
)

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
}

func NewSimulator(conf *config.Config, storeRunner *etcdstorerunner.ETCDClusterRunner, store store.Store, desiredStateServer *desiredstateserver.DesiredStateServer, cliRunner *CLIRunner, messageBus yagnats.NATSConn) *Simulator {
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
	}
}

func (s *Simulator) Tick(numTicks int) {
	timeBetweenTicks := int(s.conf.HeartbeatPeriod)

	for i := 0; i < numTicks; i++ {
		s.currentTimestamp += timeBetweenTicks
		s.storeRunner.FastForwardTime(timeBetweenTicks)
		s.sendHeartbeats()
		s.cliRunner.Run("fetch_desired", s.currentTimestamp)
		s.cliRunner.Run("analyze", s.currentTimestamp)
		s.cliRunner.Run("send", s.currentTimestamp)
	}
}

func (s *Simulator) sendHeartbeats() {
	s.cliRunner.StartListener(s.currentTimestamp)
	metronAgent.Reset()
	for _, heartbeat := range s.currentHeartbeats {
		s.messageBus.Publish("dea.heartbeat", heartbeat.ToJSON())
	}

	nHeartbeats := len(s.currentHeartbeats)
	Eventually(func() bool {
		return metronAgent.MatchEvent("listener", events.Envelope_ValueMetric, "SavedHeartbeats", float64(nHeartbeats))
	}, 5.0, 0.05).Should(BeTrue())

	s.cliRunner.StopListener()
}

func (s *Simulator) SetDesiredState(desiredStates ...models.DesiredAppState) {
	s.desiredStateServer.SetDesiredState(desiredStates)
}

func (s *Simulator) SetCurrentHeartbeats(heartbeats ...*models.Heartbeat) {
	s.currentHeartbeats = heartbeats
}
