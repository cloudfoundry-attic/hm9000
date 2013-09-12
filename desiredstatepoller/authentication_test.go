package desiredstatepoller

import (
	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Authentication", func() {
	var (
		fakeMessageBus *fake_cfmessagebus.FakeMessageBus
		poller         *desiredStatePoller
	)

	BeforeEach(func() {
		fakeMessageBus = fake_cfmessagebus.NewFakeMessageBus()
		poller = NewDesiredStatePoller(fakeMessageBus, etcdStore, desiredStateServerBaseUrl)
	})

	Context("the first time it polls", func() {
		BeforeEach(func() {
			poller.Poll()
		})

		It("should request the CC credentials over NATS", func() {
			Ω(fakeMessageBus.Requests).Should(HaveKey(authNatsSubject))
			Ω(fakeMessageBus.Requests[authNatsSubject]).Should(HaveLen(1))
			Ω(fakeMessageBus.Requests[authNatsSubject][0].Message).Should(BeEmpty())
		})

		Context("when the request arrives successfully", func() {
			BeforeEach(func() {
				fakeMessageBus.Requests[authNatsSubject][0].Callback([]byte(`{"user":"mcat","password":"testing"}`))
			})

			Context("and it polls again", func() {
				It("should not request the CC credentials again", func() {
					poller.Poll()
					Ω(fakeMessageBus.Requests[authNatsSubject]).Should(HaveLen(1))
				})
			})
		})

		Context("when the response is malformed", func() {
			BeforeEach(func() {
				fakeMessageBus.Requests[authNatsSubject][0].Callback([]byte(`{`))
			})

			Context("and it polls again", func() {
				It("should request the CC credentials again", func() {
					poller.Poll()
					Ω(fakeMessageBus.Requests[authNatsSubject]).Should(HaveLen(2))
				})
			})
		})

		Context("when the request fails to arrive", func() {
			Context("and it polls again", func() {
				It("should request the CC credentials again", func() {
					poller.Poll()
					Ω(fakeMessageBus.Requests[authNatsSubject]).Should(HaveLen(2))
				})
			})
		})
	})
})
