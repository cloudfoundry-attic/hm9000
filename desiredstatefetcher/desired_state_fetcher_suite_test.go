package desiredstatefetcher_test

import (
	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os"
	"os/signal"
	"testing"
)

const desiredStateServerBaseUrl = "http://127.0.0.1:6001"

var (
	stateServer *desiredstateserver.DesiredStateServer
	etcdRunner  *storerunner.ETCDClusterRunner
)

var _ = BeforeEach(func() {
	etcdRunner.Reset()
})

func TestDesiredStateFetcher(t *testing.T) {
	registerSignalHandler()
	RegisterFailHandler(Fail)

	fakeMessageBus := fake_cfmessagebus.NewFakeMessageBus()
	stateServer = desiredstateserver.NewDesiredStateServer(fakeMessageBus)
	go stateServer.SpinUp(6001)

	etcdRunner = storerunner.NewETCDClusterRunner(5001, 1)
	etcdRunner.Start()

	RunSpecs(t, "Desired State Fetcher Suite")

	etcdRunner.Stop()
}

func registerSignalHandler() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		select {
		case <-c:
			etcdRunner.Stop()
			os.Exit(0)
		}
	}()
}
