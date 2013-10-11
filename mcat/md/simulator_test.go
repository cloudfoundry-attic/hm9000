package md_test

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/hm9000/testhelpers/messagepublisher"
	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"
)

type Simulator struct {
	conf                   config.Config
	storeRunner            storerunner.StoreRunner
	desiredStateServer     *desiredstateserver.DesiredStateServer
	currentHeartbeats      []models.Heartbeat
	currentTimestamp       int
	cliRunner              *CLIRunner
	TicksToAttainFreshness int
	TicksToExpireHeartbeat int
	GracePeriod            int
	publisher              *messagepublisher.MessagePublisher
}

func NewSimulator(conf config.Config, storeRunner storerunner.StoreRunner, desiredStateServer *desiredstateserver.DesiredStateServer, cliRunner *CLIRunner, publisher *messagepublisher.MessagePublisher) *Simulator {
	desiredStateServer.Reset()

	return &Simulator{
		currentTimestamp:       100,
		conf:                   conf,
		storeRunner:            storeRunner,
		desiredStateServer:     desiredStateServer,
		cliRunner:              cliRunner,
		TicksToAttainFreshness: int(conf.ActualFreshnessTTLInHeartbeats) + 1,
		TicksToExpireHeartbeat: int(conf.HeartbeatTTLInHeartbeats),
		GracePeriod:            int(conf.GracePeriodInHeartbeats),
		publisher:              publisher,
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
	for _, heartbeat := range s.currentHeartbeats {
		s.publisher.PublishHeartbeat(heartbeat)
	}
	s.cliRunner.WaitForHeartbeats(len(s.currentHeartbeats))
	s.cliRunner.StopListener()
}

func (s *Simulator) SetDesiredState(desiredStates ...models.DesiredAppState) {
	s.desiredStateServer.SetDesiredState(desiredStates)
}

func (s *Simulator) SetCurrentHeartbeats(heartbeats ...models.Heartbeat) {
	s.currentHeartbeats = heartbeats
}
