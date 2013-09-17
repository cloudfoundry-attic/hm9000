package desiredstatefetcher

import (
	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/bel_air"
	"github.com/cloudfoundry/hm9000/helpers/http_client"
	"github.com/cloudfoundry/hm9000/helpers/time_provider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/test_helpers/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fetching from CC and storing the result in the Store", func() {
	var (
		conf           config.Config
		fetcher        *desiredStateFetcher
		fakeMessageBus *fake_cfmessagebus.FakeMessageBus
		a1             app.App
		a2             app.App
		a3             app.App
		resultChan     chan DesiredStateFetcherResult
	)

	BeforeEach(func() {
		resultChan = make(chan DesiredStateFetcherResult, 1)
		a1 = app.NewApp()
		a2 = app.NewApp()
		a3 = app.NewApp()

		stateServer.SetDesiredState([]models.DesiredAppState{
			a1.DesiredState(0),
			a2.DesiredState(0),
			a3.DesiredState(0),
		})
		fakeMessageBus = fake_cfmessagebus.NewFakeMessageBus()

		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())

		fetcher = NewDesiredStateFetcher(conf, fakeMessageBus, etcdStore, http_client.NewHttpClient(), bel_air.NewFreshPrince(etcdStore), &time_provider.RealTimeProvider{})
		fetcher.Fetch(resultChan)
		fakeMessageBus.Requests[conf.CCAuthMessageBusSubject][0].Callback([]byte(`{"user":"mcat","password":"testing"}`))
	})

	It("requests for the first set of data from the CC and stores the response", func() {
		node, err := etcdStore.Get("/desired/" + a1.AppGuid + "-" + a1.AppVersion)
		Ω(err).ShouldNot(HaveOccured())

		Ω(node.TTL).Should(BeNumerically("==", 10*60-1))

		Ω(node.Value).Should(Equal(a1.DesiredState(0).ToJson()))

		node, err = etcdStore.Get("/desired/" + a2.AppGuid + "-" + a2.AppVersion)
		Ω(err).ShouldNot(HaveOccured())

		Ω(node.TTL).Should(BeNumerically("==", 10*60-1))

		Ω(node.Value).Should(Equal(a2.DesiredState(0).ToJson()))

		node, err = etcdStore.Get("/desired/" + a3.AppGuid + "-" + a3.AppVersion)
		Ω(err).ShouldNot(HaveOccured())

		Ω(node.TTL).Should(BeNumerically("==", 10*60-1))

		Ω(node.Value).Should(Equal(a3.DesiredState(0).ToJson()))
	})

	It("bumps the freshness", func() {
		_, err := etcdStore.Get(conf.DesiredFreshnessKey)
		Ω(err).ShouldNot(HaveOccured())
	})

	It("reports success to the channel", func() {
		result := <-resultChan
		Ω(result.Success).Should(BeTrue())
		Ω(result.NumResults).Should(Equal(3))
		Ω(result.Message).Should(BeZero())
		Ω(result.Error).ShouldNot(HaveOccured())
	})
})
