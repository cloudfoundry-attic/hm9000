package actualstatelisteners_test

import (
	. "github.com/cloudfoundry/hm9000/actualstatelisteners"
	"github.com/cloudfoundry/hm9000/actualstatelisteners/fakes"
	. "github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"github.com/nats-io/nats"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"github.com/cloudfoundry/hm9000/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NatsListener", func() {
	var (
		dea                 DeaFixture
		listener            *NatsListener
		messageBus          *fakeyagnats.FakeNATSConn
		logger              *fakelogger.FakeLogger
		conf                *config.Config
		syncer              *fakes.FakeSyncer
		natsListenerProcess ifrit.Process
		topic               string
	)

	beat := func(t string) {
		messageBus.SubjectCallbacks(t)[0](&nats.Msg{
			Data: dea.Heartbeat(1).ToJSON(),
		})
	}

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()

		Expect(err).NotTo(HaveOccurred())

		messageBus = fakeyagnats.Connect()
		logger = fakelogger.NewFakeLogger()
		syncer = &fakes.FakeSyncer{}
		dea = NewDeaFixture()
		topic = "dea.heartbeat"
	})

	JustBeforeEach(func() {
		listener = NewNatsListener(logger, conf, messageBus, syncer)
		natsListenerProcess = ifrit.Background(listener)
		Eventually(natsListenerProcess.Ready()).Should(BeClosed())
	})

	AfterEach(func() {
		ginkgomon.Kill(natsListenerProcess)
	})

	It("To subscribe to the dea.heartbeat subject", func() {
		Expect(messageBus.Subscriptions("dea.heartbeat")).To(HaveLen(1))
		Expect(messageBus.SubjectCallbacks(topic)).To(HaveLen(1))
	})

	It("sends the heartbeat to the syncer", func() {
		beat(topic)

		Expect(syncer.HeartbeatCallCount()).To(Equal(1))
		Expect(syncer.HeartbeatArgsForCall(0)).To(Equal(dea.Heartbeat(1)))
	})

	Context("When it fails to parse the heartbeat message", func() {
		JustBeforeEach(func() {
			messageBus.SubjectCallbacks("dea.heartbeat")[0](&nats.Msg{
				Data: []byte("ß"),
			})
		})

		It("does not send the heartbeat to the syncer", func() {
			Expect(syncer.HeartbeatCallCount()).To(Equal(0))
		})

		It("logs about the failed parse", func() {
			Eventually(logger.LoggedSubjects).Should(ContainElement("Failed to unmarshal dea heartbeat"))
		})
	})
})
