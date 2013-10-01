package md_test

import (
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/messagepublisher"
	"github.com/cloudfoundry/hm9000/testhelpers/startstoplistener"
	. "github.com/onsi/ginkgo"
	ginkgoConfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"time"

	"github.com/cloudfoundry/hm9000/config"
	. "github.com/cloudfoundry/hm9000/mcat/md"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/hm9000/testhelpers/natsrunner"
	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"
	"os"
	"os/signal"
	"testing"
)

const desiredStateServerBaseUrl = "http://127.0.0.1:6001"
const natsPort = 4223

var (
	stateServer       *desiredstateserver.DesiredStateServer
	storeRunner       storerunner.StoreRunner
	storeAdapter      storeadapter.StoreAdapter
	natsRunner        *natsrunner.NATSRunner
	conf              config.Config
	cliRunner         *CLIRunner
	publisher         *messagepublisher.MessagePublisher
	startStopListener *startstoplistener.StartStopListener
)

func TestMd(t *testing.T) {
	registerSignalHandler()
	RegisterFailHandler(Fail)

	natsRunner = natsrunner.NewNATSRunner(natsPort)
	natsRunner.Start()

	stateServer = desiredstateserver.NewDesiredStateServer(natsRunner.MessageBus)
	go stateServer.SpinUp(6001)

	var err error
	conf, err = config.DefaultConfig()
	Ω(err).ShouldNot(HaveOccured())

	//for now, run the suite for ETCD...
	startEtcd()
	publisher = messagepublisher.NewMessagePublisher(natsRunner.MessageBus)
	startStopListener = startstoplistener.NewStartStopListener(natsRunner.MessageBus, conf)

	cliRunner = NewCLIRunner(storeRunner.NodeURLS(), desiredStateServerBaseUrl, natsPort, ginkgoConfig.DefaultReporterConfig.Verbose)

	RunSpecs(t, "Md Suite (ETCD)")

	storeAdapter.Disconnect()
	stopAllExternalProcesses()

	//...and then for zookeeper
	// startZookeeper()

	// RunSpecs(t, "Md Suite (Zookeeper)")

	// storeAdapter.Disconnect()
	// storeRunner.Stop()
}

var _ = BeforeEach(func() {
	storeRunner.Reset()
	startStopListener.Reset()
})

func stopAllExternalProcesses() {
	storeRunner.Stop()
	natsRunner.Stop()
	cliRunner.Cleanup()
}

func startEtcd() {
	etcdPort := 5000 + (ginkgoConfig.GinkgoConfig.ParallelNode-1)*10
	storeRunner = storerunner.NewETCDClusterRunner(etcdPort, 1)
	storeRunner.Start()

	storeAdapter = storeadapter.NewETCDStoreAdapter(storeRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
	err := storeAdapter.Connect()
	Ω(err).ShouldNot(HaveOccured())
}

func startZookeeper() {
	zookeeperPort := 2181 + (ginkgoConfig.GinkgoConfig.ParallelNode-1)*10
	storeRunner = storerunner.NewZookeeperClusterRunner(zookeeperPort, 1)
	storeRunner.Start()

	storeAdapter = storeadapter.NewZookeeperStoreAdapter(storeRunner.NodeURLS(), conf.StoreMaxConcurrentRequests, &timeprovider.RealTimeProvider{}, time.Second)
	err := storeAdapter.Connect()
	Ω(err).ShouldNot(HaveOccured())
}

func sendHeartbeats(timestamp int, heartbeats []models.Heartbeat, times int, deltaT int) int {
	nodes, err := storeAdapter.List("/actual")
	if err != storeadapter.ErrorKeyNotFound {
		Ω(err).ShouldNot(HaveOccured())
		for _, node := range nodes {
			err := storeAdapter.Delete(node.Key)
			Ω(err).ShouldNot(HaveOccured())
		}
	}

	for i := 0; i < times; i++ {
		cliRunner.StartListener(timestamp)
		for _, heartbeat := range heartbeats {
			publisher.PublishHeartbeat(heartbeat)
		}
		cliRunner.WaitForHeartbeats(len(heartbeats))
		cliRunner.StopListener()
		timestamp += deltaT
	}
	return timestamp
}

func registerSignalHandler() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		select {
		case <-c:
			stopAllExternalProcesses()
			os.Exit(0)
		}
	}()
}
