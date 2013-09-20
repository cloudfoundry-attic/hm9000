package md_test

import (
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	. "github.com/onsi/ginkgo"
	ginkgoConfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"time"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/hm9000/testhelpers/natsrunner"
	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"
	"os"
	"os/signal"
	"testing"
)

const desiredStateServerBaseUrl = "http://127.0.0.1:6001"

var (
	stateServer  *desiredstateserver.DesiredStateServer
	storeRunner  storerunner.StoreRunner
	storeAdapter storeadapter.StoreAdapter
	natsRunner   *natsrunner.NATSRunner
	conf         config.Config
)

func TestMd(t *testing.T) {
	registerSignalHandler()
	RegisterFailHandler(Fail)

	natsRunner = natsrunner.NewNATSRunner(4223)
	natsRunner.Start()

	stateServer = desiredstateserver.NewDesiredStateServer(natsRunner.MessageBus)
	go stateServer.SpinUp(6001)

	var err error
	conf, err = config.DefaultConfig()
	Ω(err).ShouldNot(HaveOccured())

	//for now, run the suite for ETCD...
	startEtcd()

	RunSpecs(t, "Md Suite (ETCD)")

	storeAdapter.Disconnect()
	storeRunner.Stop()

	//...and then for zookeeper
	startZookeeper()

	RunSpecs(t, "Md Suite (Zookeeper)")

	storeAdapter.Disconnect()
	storeRunner.Stop()
}

var _ = BeforeEach(func() {
	storeRunner.Reset()
})

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

func registerSignalHandler() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		select {
		case <-c:
			storeRunner.Stop()
			os.Exit(0)
		}
	}()
}
