package phd

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"

	"os"
	"os/signal"
	"testing"
)

var etcdRunner *storerunner.ETCDClusterRunner

func TestPhd(t *testing.T) {
	registerSignalHandler()
	RegisterFailHandler(Fail)

	RunSpecs(t, "Phd Performance Suite")

	if etcdRunner != nil {
		etcdRunner.Stop()
	}
}

func registerSignalHandler() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, os.Kill)

		select {
		case <-c:
			if etcdRunner != nil {
				etcdRunner.Stop()
			}
			os.Exit(0)
		}
	}()
}
