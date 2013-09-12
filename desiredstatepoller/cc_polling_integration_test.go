package desiredstatepoller

import (
	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	"github.com/cloudfoundry/hm9000/helpers/http_client"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/test_helpers/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Polling CC and storing the result in the Store", func() {
	var (
		poller         *desiredStatePoller
		fakeMessageBus *fake_cfmessagebus.FakeMessageBus
		a1             app.App
		a2             app.App
	)

	BeforeEach(func() {
		a1 = app.NewApp()
		a2 = app.NewApp()

		stateServer.SetDesiredState([]models.DesiredAppState{
			a1.DesiredState(0),
			a2.DesiredState(0),
		})
		fakeMessageBus = fake_cfmessagebus.NewFakeMessageBus()
		poller = NewDesiredStatePoller(fakeMessageBus, etcdStore, &http_client.RealHttpClientFactory{}, desiredStateServerBaseUrl)
		poller.Poll()
		fakeMessageBus.Requests[authNatsSubject][0].Callback([]byte(`{"user":"mcat","password":"testing"}`))
	})

	It("requests for the first set of data from the CC and stores the response", func() {
		node, err := etcdStore.Get("/desired/" + a1.AppGuid + "-" + a1.AppVersion)
		Ω(err).ShouldNot(HaveOccured())

		Ω(node.TTL).Should(BeNumerically("<=", 10*60))
		Ω(node.TTL).Should(BeNumerically(">", 10*60-5))

		Ω(node.Value).Should(Equal(a1.DesiredState(0).ToJson()))

		node, err = etcdStore.Get("/desired/" + a2.AppGuid + "-" + a2.AppVersion)
		Ω(err).ShouldNot(HaveOccured())

		Ω(node.TTL).Should(BeNumerically("<=", 10*60))
		Ω(node.TTL).Should(BeNumerically(">", 10*60-5))

		Ω(node.Value).Should(Equal(a2.DesiredState(0).ToJson()))
	})
})
