package phd

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"

	"os"
	"os/signal"
	"testing"
)

var etcdRunner *etcd_runner.ETCDClusterRunner

func TestBootstrap(t *testing.T) {
	registerSignalHandler()
	RegisterFailHandler(Fail)

	nodes := 5
	etcdRunner = etcd_runner.NewETCDClusterRunner("etcd", 5001, nodes)
	etcdRunner.Start()

	RunSpecs(t, fmt.Sprintf("Phd Suite With %d ETCD nodes", nodes))

	etcdRunner.Stop()
}

var _ = BeforeEach(func() {
	etcdRunner.Stop()
	etcdRunner.Start()
})

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
