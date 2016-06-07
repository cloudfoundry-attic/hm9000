package mcat_test

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/cloudfoundry-incubator/consuladapter/consulrunner"
	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/hm9000/config"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/natsrunner"
	"github.com/cloudfoundry/hm9000/testhelpers/startstoplistener"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/storerunner/etcdstorerunner"
	"github.com/cloudfoundry/yagnats"
	"github.com/gogo/protobuf/proto"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type eventTracker struct {
	origin    string
	eventType string
	name      string
	value     float64
}

type Metron struct {
	udpListener    net.PacketConn
	lock           sync.RWMutex
	receivedEvents []eventTracker
}

type MCATCoordinator struct {
	MessageBus   yagnats.NATSConn
	StateServer  *desiredstateserver.DesiredStateServer
	StoreRunner  *etcdstorerunner.ETCDClusterRunner
	StoreAdapter *etcdstoreadapter.ETCDStoreAdapter

	hm9000Binary      string
	natsRunner        *natsrunner.NATSRunner
	startStopListener *startstoplistener.StartStopListener
	metron            *Metron

	Conf *config.Config

	DesiredStateServerBaseUrl string
	DesiredStateServerPort    int
	NatsPort                  int
	NatsMonitoringPort        int
	DropsondePort             int

	ParallelNode int
	Verbose      bool

	currentCLIRunner *CLIRunner

	ConsulRunner *consulrunner.ClusterRunner
}

func NewMCATCoordinator(hm9000Binary string, parallelNode int, verbose bool) *MCATCoordinator {
	coordinator := &MCATCoordinator{
		hm9000Binary: hm9000Binary,
		ParallelNode: parallelNode,
		Verbose:      verbose,
	}
	coordinator.loadConfig()
	coordinator.computePorts()

	return coordinator
}

func (coordinator *MCATCoordinator) loadConfig() {
	conf, err := config.DefaultConfig()
	Expect(err).ToNot(HaveOccurred())
	coordinator.Conf = conf
}

func (coordinator *MCATCoordinator) computePorts() {
	coordinator.DesiredStateServerPort = 6001 + coordinator.ParallelNode
	coordinator.DesiredStateServerBaseUrl = "http://127.0.0.1:" + strconv.Itoa(coordinator.DesiredStateServerPort)
	coordinator.NatsPort = 4223 + coordinator.ParallelNode
	coordinator.NatsMonitoringPort = 8223 + coordinator.ParallelNode
	coordinator.DropsondePort = 7879 + coordinator.ParallelNode
}

func (coordinator *MCATCoordinator) PrepForNextTest() (*CLIRunner, *Simulator, *startstoplistener.StartStopListener, *Metron) {
	coordinator.StoreRunner.Reset()
	coordinator.startStopListener.Reset()
	coordinator.StateServer.Reset()
	coordinator.metron.Reset()

	if coordinator.currentCLIRunner != nil {
		coordinator.currentCLIRunner.Cleanup()
	}
	coordinator.currentCLIRunner = NewCLIRunner(coordinator.hm9000Binary, coordinator.StoreRunner.NodeURLS(), coordinator.DesiredStateServerBaseUrl, coordinator.NatsPort, coordinator.DropsondePort, coordinator.ConsulRunner.ConsulCluster(), coordinator.Conf.CCInternalURL, coordinator.Verbose)
	store := storepackage.NewStore(coordinator.Conf, coordinator.StoreAdapter, fakelogger.NewFakeLogger())
	simulator := NewSimulator(coordinator.Conf, coordinator.StoreRunner, store, coordinator.StateServer, coordinator.currentCLIRunner, coordinator.MessageBus, coordinator.NatsMonitoringPort)

	return coordinator.currentCLIRunner, simulator, coordinator.startStopListener, coordinator.metron
}

func (coordinator *MCATCoordinator) StartNats() {
	coordinator.natsRunner = natsrunner.NewNATSRunner(coordinator.NatsPort, coordinator.NatsMonitoringPort)
	coordinator.natsRunner.Start()
	coordinator.MessageBus = coordinator.natsRunner.MessageBus
}

func (coordinator *MCATCoordinator) StartDesiredStateServer() {
	coordinator.StateServer = desiredstateserver.NewDesiredStateServer(coordinator.DesiredStateServerPort)
	go coordinator.StateServer.SpinUp()
}

func (coordinator *MCATCoordinator) StartStartStopListener() {
	coordinator.startStopListener, coordinator.Conf.CCInternalURL = startstoplistener.NewStartStopListener(coordinator.MessageBus, coordinator.Conf)
}

func (coordinator *MCATCoordinator) StartETCD() {
	etcdPort := 5000 + (coordinator.ParallelNode-1)*10
	coordinator.StoreRunner = etcdstorerunner.NewETCDClusterRunner(etcdPort, 1, nil)
	coordinator.StoreRunner.Start()

	pool, err := workpool.NewWorkPool(coordinator.Conf.StoreMaxConcurrentRequests)
	Expect(err).NotTo(HaveOccurred())

	coordinator.StoreAdapter, err = etcdstoreadapter.New(&etcdstoreadapter.ETCDOptions{ClusterUrls: coordinator.StoreRunner.NodeURLS()}, pool)
	Expect(err).NotTo(HaveOccurred())
	err = coordinator.StoreAdapter.Connect()
	Expect(err).NotTo(HaveOccurred())
}

func (coordinator *MCATCoordinator) StopETCD() {
	coordinator.StoreRunner.Stop()
	if coordinator.StoreAdapter != nil {
		coordinator.StoreAdapter.Disconnect()
	}
}

func (coordinator *MCATCoordinator) StartConsulRunner() {
	consulPort := 10000 + (coordinator.ParallelNode-1)*10
	coordinator.ConsulRunner = consulrunner.NewClusterRunner(consulPort, 1, "http")
	coordinator.ConsulRunner.Start()
	coordinator.ConsulRunner.WaitUntilReady()
}

func (coordinator *MCATCoordinator) StopConsulRunner() {
	coordinator.ConsulRunner.Stop()
}

func (coordinator *MCATCoordinator) ResetConsulRunner() {
	coordinator.ConsulRunner.Reset()
	coordinator.ConsulRunner.WaitUntilReady()
}

func (coordinator *MCATCoordinator) StopAllExternalProcesses() {
	coordinator.StoreRunner.Stop()
	coordinator.natsRunner.Stop()

	if coordinator.currentCLIRunner != nil {
		coordinator.currentCLIRunner.Cleanup()
	}
}

func (coordinator *MCATCoordinator) StartMetron() *Metron {
	udpListener, err := net.ListenPacket("udp4", fmt.Sprintf(":%d", coordinator.DropsondePort))
	Expect(err).ToNot(HaveOccurred())

	m := &Metron{
		udpListener: udpListener,
	}

	go func() {
		defer ginkgo.GinkgoRecover()
		m.listenForEvents()
	}()

	coordinator.metron = m
	return m
}

func (metron *Metron) Stop() {
	metron.udpListener.Close()
}

func (metron *Metron) Reset() {
	metron.lock.Lock()
	metron.receivedEvents = nil
	metron.lock.Unlock()
}

func (metron *Metron) MatchEvent(origin string, eventType events.Envelope_EventType, name string, value float64) bool {
	metron.lock.RLock()
	defer metron.lock.RUnlock()

	matching := eventTracker{
		origin:    origin,
		eventType: proto.EnumName(events.Envelope_EventType_name, int32(eventType)),
		name:      name,
		value:     value,
	}
	for _, e := range metron.receivedEvents {
		if e == matching {
			return true
		}
	}

	return false
}

func (metron *Metron) listenForEvents() {
	for {
		buffer := make([]byte, 1024)
		n, _, err := metron.udpListener.ReadFrom(buffer)
		if err != nil {
			return
		}

		Expect(n).ToNot(Equal(0), "Received empty packet")
		envelope := new(events.Envelope)
		err = proto.Unmarshal(buffer[0:n], envelope)
		Expect(err).ToNot(HaveOccurred())

		var eventId = envelope.GetEventType().String()

		tracker := eventTracker{origin: envelope.GetOrigin(), eventType: eventId}

		switch envelope.GetEventType() {
		case events.Envelope_ValueMetric:
			tracker.name = envelope.GetValueMetric().GetName()
			tracker.value = envelope.GetValueMetric().GetValue()
		case events.Envelope_CounterEvent:
			tracker.name = envelope.GetCounterEvent().GetName()
			tracker.value = float64(envelope.GetCounterEvent().GetDelta())
		default:
			continue
		}

		metron.lock.Lock()
		metron.receivedEvents = append(metron.receivedEvents, tracker)
		metron.lock.Unlock()
	}
}
