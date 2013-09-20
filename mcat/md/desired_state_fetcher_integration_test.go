package md_test

import (
	"github.com/cloudfoundry/hm9000/desiredstatefetcher"
	"github.com/cloudfoundry/hm9000/helpers/httpclient"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fetching from CC and storing the result in the Store", func() {
	var (
		fetcher    *desiredstatefetcher.DesiredStateFetcher
		a1         app.App
		a2         app.App
		a3         app.App
		resultChan chan desiredstatefetcher.DesiredStateFetcherResult
	)

	BeforeEach(func() {
		resultChan = make(chan desiredstatefetcher.DesiredStateFetcherResult, 1)
		a1 = app.NewApp()
		a2 = app.NewApp()
		a3 = app.NewApp()

		stateServer.SetDesiredState([]models.DesiredAppState{
			a1.DesiredState(0),
			a2.DesiredState(0),
			a3.DesiredState(0),
		})

		fetcher = desiredstatefetcher.New(conf, natsRunner.MessageBus, store.NewStore(conf, storeAdapter), httpclient.NewHttpClient(), &timeprovider.RealTimeProvider{})
		fetcher.Fetch(resultChan)
	})

	It("requests for the first set of data from the CC and stores the response", func() {
		var node storeadapter.StoreNode
		var err error
		Eventually(func() error {
			node, err = storeAdapter.Get("/desired/" + a1.AppGuid + "-" + a1.AppVersion)
			return err
		}, 1, 0.1).ShouldNot(HaveOccured())

		Ω(node.TTL).Should(BeNumerically("<=", 10*60))
		Ω(node.TTL).Should(BeNumerically(">=", 10*60-1))

		Ω(node.Value).Should(Equal(a1.DesiredState(0).ToJson()))

		node, err = storeAdapter.Get("/desired/" + a2.AppGuid + "-" + a2.AppVersion)
		Ω(err).ShouldNot(HaveOccured())

		Ω(node.TTL).Should(BeNumerically("<=", 10*60))
		Ω(node.TTL).Should(BeNumerically(">=", 10*60-1))

		Ω(node.Value).Should(Equal(a2.DesiredState(0).ToJson()))

		node, err = storeAdapter.Get("/desired/" + a3.AppGuid + "-" + a3.AppVersion)
		Ω(err).ShouldNot(HaveOccured())

		Ω(node.TTL).Should(BeNumerically("<=", 10*60))
		Ω(node.TTL).Should(BeNumerically(">=", 10*60-1))

		Ω(node.Value).Should(Equal(a3.DesiredState(0).ToJson()))
	})

	It("bumps the freshness", func() {
		Eventually(func() error {
			_, err := storeAdapter.Get(conf.DesiredFreshnessKey)
			return err
		}, 1, 0.1).ShouldNot(HaveOccured())
	})

	It("reports success to the channel", func() {
		result := <-resultChan
		Ω(result.Success).Should(BeTrue())
		Ω(result.NumResults).Should(Equal(3))
		Ω(result.Message).Should(BeZero())
		Ω(result.Error).ShouldNot(HaveOccured())
	})
})
