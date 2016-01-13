package mcat_test

import (
	"testing"

	"github.com/cloudfoundry/hm9000/testhelpers/startstoplistener"
	. "github.com/onsi/ginkgo"
	ginkgoConfig "github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var (
	coordinator       *MCATCoordinator
	simulator         *Simulator
	cliRunner         *CLIRunner
	startStopListener *startstoplistener.StartStopListener
	metronAgent       *Metron
)

func TestMCAT(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "MCAT ETCD MD Suite")
}

var _ = BeforeSuite(func() {
	hm9000Binary, err := gexec.Build("github.com/cloudfoundry/hm9000")
	Expect(err).ToNot(HaveOccurred())

	coordinator = NewMCATCoordinator(hm9000Binary, ginkgoConfig.GinkgoConfig.ParallelNode, ginkgoConfig.DefaultReporterConfig.Verbose)

	coordinator.StartNats()
	coordinator.StartDesiredStateServer()
	coordinator.StartStartStopListener()
	coordinator.StartMetron()
	coordinator.StartETCD()
})

var _ = BeforeEach(func() {
	cliRunner, simulator, startStopListener, metronAgent = coordinator.PrepForNextTest()
})

var _ = AfterSuite(func() {
	coordinator.StopETCD()
	coordinator.StopAllExternalProcesses()
	gexec.CleanupBuildArtifacts()
})
