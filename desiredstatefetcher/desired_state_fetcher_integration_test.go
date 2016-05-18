package desiredstatefetcher_test

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/desiredstatefetcher"
	"github.com/cloudfoundry/hm9000/helpers/httpclient"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/clock"
)

var _ = Describe("Fetching from CC and storing the result in the Store", func() {
	var (
		fetcher    *desiredstatefetcher.DesiredStateFetcher
		a1         appfixture.AppFixture
		a2         appfixture.AppFixture
		a3         appfixture.AppFixture
		resultChan chan desiredstatefetcher.DesiredStateFetcherResult
		conf       *config.Config
		appQueue   *models.AppQueue
	)

	BeforeEach(func() {
		resultChan = make(chan desiredstatefetcher.DesiredStateFetcherResult, 1)
		a1 = appfixture.NewAppFixture()
		a2 = appfixture.NewAppFixture()
		a3 = appfixture.NewAppFixture()

		stateServer.SetDesiredState([]models.DesiredAppState{
			a1.DesiredState(1),
			a2.DesiredState(1),
			a3.DesiredState(1),
		})

		conf, _ = config.DefaultConfig()
		conf.CCBaseURL = stateServer.URL()

		appQueue = models.NewAppQueue()

		fetcher = desiredstatefetcher.New(conf,
			httpclient.NewHttpClient(conf.SkipSSLVerification, conf.FetcherNetworkTimeout()),
			clock.NewClock(),
			fakelogger.NewFakeLogger(),
		)
		fetcher.Fetch(resultChan, appQueue)
	})

	It("requests for the first set of data from the CC and sends the response", func() {
		var desired map[string]models.DesiredAppState

		Eventually(appQueue.DesiredApps).Should(Receive(&desired))

		Expect(desired).To(HaveKey(a1.AppGuid + "," + a1.AppVersion))
		Expect(desired).To(HaveKey(a2.AppGuid + "," + a2.AppVersion))
		Expect(desired).To(HaveKey(a3.AppGuid + "," + a3.AppVersion))
	})

	It("reports success to the channel", func() {
		var result desiredstatefetcher.DesiredStateFetcherResult
		Eventually(resultChan).Should(Receive(&result))
		Expect(result.Success).To(BeTrue())
		Expect(result.NumResults).To(Equal(3))
		Expect(result.Message).To(BeZero())
		Expect(result.Error).NotTo(HaveOccurred())
	})

	Context("when fetching again, and apps have been stopped and/or deleted", func() {
		BeforeEach(func() {
			Eventually(resultChan).Should(Receive())

			desired1 := a1.DesiredState(1)
			desired1.State = models.AppStateStopped

			stateServer.SetDesiredState([]models.DesiredAppState{
				desired1,
				a3.DesiredState(1),
			})

			fetcher.Fetch(resultChan, appQueue)
		})
	})
})
