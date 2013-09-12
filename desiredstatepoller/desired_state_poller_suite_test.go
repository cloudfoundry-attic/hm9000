package desiredstatepoller

import (
	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/test_helpers/desired_state_server"
	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os"
	"os/signal"
	"testing"
)

const authNatsSubject = "cloudcontroller.bulk.credentials.default"
const desiredStateServerBaseUrl = "http://127.0.0.1:6001"
const etcdPort = 4001

var (
	stateServer *desired_state_server.DesiredStateServer
	etcdRunner  *etcd_runner.ETCDRunner
	etcdStore   store.Store
)

var _ = BeforeEach(func() {
	etcdRunner.StopETCD()
	etcdRunner.StartETCD()

	etcdStore = store.NewETCDStore(config.ETCD_URL(etcdPort))
	err := etcdStore.Connect()
	Ω(err).ShouldNot(HaveOccured())
})

func TestBootstrap(t *testing.T) {
	registerSignalHandler()
	RegisterFailHandler(Fail)

	fakeMessageBus := fake_cfmessagebus.NewFakeMessageBus()
	stateServer = desired_state_server.NewDesiredStateServer(fakeMessageBus)
	go stateServer.SpinUp(6001)

	etcdRunner = etcd_runner.NewETCDRunner("etcd", etcdPort)

	RunSpecs(t, "Desired State Poller")

	etcdRunner.StopETCD()
}

func registerSignalHandler() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		select {
		case <-c:
			etcdRunner.StopETCD()
			os.Exit(0)
		}
	}()
}
