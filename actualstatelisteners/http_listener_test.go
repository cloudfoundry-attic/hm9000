package actualstatelisteners_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	. "github.com/cloudfoundry/hm9000/actualstatelisteners"
	"github.com/cloudfoundry/hm9000/actualstatelisteners/fakes"
	"github.com/cloudfoundry/hm9000/config"
	. "github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HttpListener", func() {
	var (
		listener         http.Handler
		logger           *fakelogger.FakeLogger
		conf             *config.Config
		syncer           *fakes.FakeSyncer
		dea              DeaFixture
		heartbeatRequest *http.Request
		err              error
		response         *httptest.ResponseRecorder
	)

	beat := func(response *httptest.ResponseRecorder, request *http.Request) {
		listener.ServeHTTP(response, request)
	}

	BeforeEach(func() {
		conf, err = config.DefaultConfig()
		Expect(err).NotTo(HaveOccurred())

		logger = fakelogger.NewFakeLogger()
		syncer = &fakes.FakeSyncer{}
		dea = NewDeaFixture()

		heartbeatRequest, err = http.NewRequest(
			"POST",
			"/dea/heartbeat",
			bytes.NewBuffer(dea.Heartbeat(1).ToJSON()),
		)

		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		listener, err = NewHttpListener(logger, conf, syncer)
		Expect(err).NotTo(HaveOccurred())
		response = httptest.NewRecorder()
	})

	It("accepts heartbeats over http", func() {
		beat(response, heartbeatRequest)
		Expect(response.Code).To(Equal(202))
	})

	It("sends the heartbeat to the syncer", func() {
		beat(response, heartbeatRequest)

		Expect(syncer.HeartbeatCallCount()).To(Equal(1))
		Expect(syncer.HeartbeatArgsForCall(0)).To(Equal(dea.Heartbeat(1)))
	})

	Context("when it fails to parse the body", func() {
		JustBeforeEach(func() {
			heartbeatRequest, err = http.NewRequest(
				"POST",
				"/dea/heartbeat",
				bytes.NewBuffer([]byte{}),
			)
			beat(response, heartbeatRequest)
		})

		It("does not send the heartbeat to the syncer", func() {
			Expect(syncer.HeartbeatCallCount()).To(Equal(0))
		})

		It("logs about the failed parse", func() {
			Eventually(logger.LoggedSubjects).Should(ContainElement("Failed to read dea heartbeat body"))
		})

		It("returns a 400 Bad Request", func() {
			Expect(response.Code).To(Equal(400))
		})
	})

	Context("when it fails to unmarshal the heartbeat message", func() {
		JustBeforeEach(func() {
			heartbeatRequest, err = http.NewRequest(
				"POST",
				"/dea/heartbeat",
				bytes.NewBuffer([]byte("ß")),
			)
			beat(response, heartbeatRequest)
		})

		It("does not send the heartbeat to the syncer", func() {
			Expect(syncer.HeartbeatCallCount()).To(Equal(0))
		})

		It("logs about the failed parse", func() {
			Eventually(logger.LoggedSubjects).Should(ContainElement("Failed to unmarshal dea heartbeat"))
		})

		It("returns a 400 Bad Request", func() {
			Expect(response.Code).To(Equal(400))
		})
	})
})
