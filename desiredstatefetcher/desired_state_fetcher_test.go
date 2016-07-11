package desiredstatefetcher_test

import (
	"errors"
	"fmt"

	"github.com/cloudfoundry/hm9000/config"
	. "github.com/cloudfoundry/hm9000/desiredstatefetcher"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	"code.cloudfoundry.org/clock/fakeclock"

	"time"

	"github.com/cloudfoundry/hm9000/testhelpers/fakehttpclient"

	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type brokenReader struct{}

func (b *brokenReader) Read([]byte) (int, error) {
	return 0, errors.New("oh no you didn't!")
}

func (b *brokenReader) Close() error {
	return nil
}

var _ = Describe("DesiredStateFetcher", func() {
	var (
		conf         *config.Config
		fetcher      *DesiredStateFetcher
		httpClient   *fakehttpclient.FakeHttpClient
		timeProvider *fakeclock.FakeClock
		storeAdapter *fakestoreadapter.FakeStoreAdapter
		resultChan   chan DesiredStateFetcherResult
		appQueue     *models.AppQueue
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Expect(err).ToNot(HaveOccurred())

		resultChan = make(chan DesiredStateFetcherResult, 1)
		timeProvider = fakeclock.NewFakeClock(time.Unix(100, 0))

		httpClient = fakehttpclient.NewFakeHttpClient()
		storeAdapter = fakestoreadapter.New()

		appQueue = models.NewAppQueue()

		fetcher = New(conf, httpClient, timeProvider, fakelogger.NewFakeLogger())
	})

	JustBeforeEach(func() {
		fetcher.Fetch(resultChan, appQueue)
	})

	AfterEach(func() {

	})

	Describe("Fetching with an invalid URL", func() {
		BeforeEach(func() {
			conf.CCBaseURL = "http://example.com/#%ZZ"
			fetcher = New(conf, httpClient, timeProvider, fakelogger.NewFakeLogger())
		})

		It("should send an error down the result channel", func() {
			var result DesiredStateFetcherResult
			Eventually(resultChan).Should(Receive(&result))
			Expect(result.Success).To(BeFalse())
			Expect(result.Message).To(Equal("Failed to generate URL request"))
			Expect(result.Error).To(HaveOccurred())
		})
	})

	Describe("Fetching batches", func() {
		var response DesiredStateServerResponse

		It("should make the correct request", func() {
			Expect(httpClient.Requests).To(HaveLen(1))
			request := httpClient.Requests[0]

			Expect(request.URL.String()).To(ContainSubstring(conf.CCBaseURL))
			Expect(request.URL.Path).To(ContainSubstring("/bulk/apps"))

			expectedAuth := models.BasicAuthInfo{
				User:     "mcat",
				Password: "testing",
			}.Encode()

			Expect(request.Header.Get("Authorization")).To(Equal(expectedAuth))
		})

		It("should request a batch size with an empty bulk token", func() {
			query := httpClient.LastRequest().URL.Query()
			Expect(query.Get("batch_size")).To(Equal(fmt.Sprintf("%d", conf.DesiredStateBatchSize)))
			Expect(query.Get("bulk_token")).To(Equal("{}"))
		})

		assertFailure := func(expectedMessage string, numRequests int) {
			It("should stop requesting batches", func() {
				Expect(httpClient.Requests).To(HaveLen(numRequests))
			})

			It("should send an error down the result channel", func() {
				var result DesiredStateFetcherResult
				Eventually(resultChan).Should(Receive(&result))
				Expect(result.Success).To(BeFalse())
				Expect(result.Message).To(Equal(expectedMessage))
				Expect(result.Error).To(HaveOccurred())
			})
		}

		Context("when a response with desired state is received", func() {
			var (
				a1                appfixture.AppFixture
				a2                appfixture.AppFixture
				stoppedApp        appfixture.AppFixture
				pendingStagingApp appfixture.AppFixture
				failedToStageApp  appfixture.AppFixture
				deletedApp        appfixture.AppFixture

				pendingStagingDesiredState models.DesiredAppState
			)

			BeforeEach(func() {
				deletedApp = appfixture.NewAppFixture()

				a1 = appfixture.NewAppFixture()
				a2 = appfixture.NewAppFixture()

				stoppedApp = appfixture.NewAppFixture()
				stoppedDesiredState := stoppedApp.DesiredState(1)
				stoppedDesiredState.State = models.AppStateStopped

				pendingStagingApp = appfixture.NewAppFixture()
				pendingStagingDesiredState = pendingStagingApp.DesiredState(1)
				pendingStagingDesiredState.PackageState = models.AppPackageStatePending

				failedToStageApp = appfixture.NewAppFixture()
				failedStagingDesiredState := failedToStageApp.DesiredState(1)
				failedStagingDesiredState.PackageState = models.AppPackageStateFailed

				response = DesiredStateServerResponse{
					Results: map[string]models.DesiredAppState{
						a1.AppGuid:                a1.DesiredState(1),
						a2.AppGuid:                a2.DesiredState(1),
						stoppedApp.AppGuid:        stoppedDesiredState,
						pendingStagingApp.AppGuid: pendingStagingDesiredState,
						failedToStageApp.AppGuid:  failedStagingDesiredState,
					},
					BulkToken: BulkToken{
						Id: 5,
					},
				}
			})

			JustBeforeEach(func() {
				httpClient.LastRequest().Succeed(response.ToJSON())
			})

			It("should request the next batch", func() {
				Expect(httpClient.Requests).To(HaveLen(2))
				Expect(httpClient.LastRequest().URL.Query().Get("bulk_token")).To(Equal(response.BulkTokenRepresentation()))
			})

			It("should not send a result down the resultChan yet", func() {
				Expect(resultChan).To(HaveLen(0))
			})

			It("should send a batch of desired state down the appQueue", func() {
				Expect(appQueue.DesiredApps).To(HaveLen(1))
			})

			Context("when an empty response is received", func() {
				JustBeforeEach(func() {
					response = DesiredStateServerResponse{
						Results: map[string]models.DesiredAppState{},
						BulkToken: BulkToken{
							Id: 17,
						},
					}

					httpClient.LastRequest().Succeed(response.ToJSON())
				})

				It("should stop requesting batches", func() {
					Expect(httpClient.Requests).To(HaveLen(2))
				})

				It("has sent all the desired apps state on the appQueue's DesiredApps channel", func() {
					var desired map[string]models.DesiredAppState

					Eventually(appQueue.DesiredApps).Should(Receive(&desired))
					Expect(desired).To(HaveLen(3))
					Expect(desired).To(ContainElement(EqualDesiredState(a1.DesiredState(1))))
					Expect(desired).To(ContainElement(EqualDesiredState(a2.DesiredState(1))))
					Expect(desired).To(ContainElement(EqualDesiredState(pendingStagingDesiredState)))
				})

				It("should send a succesful result down the result channel", func() {
					var result DesiredStateFetcherResult
					Eventually(resultChan).Should(Receive(&result))
					Expect(result.Success).To(BeTrue())
					Expect(result.Message).To(BeZero())
					Expect(result.Error).ToNot(HaveOccurred())
				})
			})
		})

		Context("when the analyzer encountered is not analyzing", func() {
			BeforeEach(func() {
				close(appQueue.DoneAnalyzing)
				a1 := appfixture.NewAppFixture()

				response = DesiredStateServerResponse{
					Results: map[string]models.DesiredAppState{
						a1.AppGuid: a1.DesiredState(1),
					},
					BulkToken: BulkToken{
						Id: 1,
					},
				}

				// Fill up the app queue with info so sendBatch cannot send
				appQueue.DesiredApps <- map[string]models.DesiredAppState{}
			})

			JustBeforeEach(func() {
				httpClient.LastRequest().Succeed(response.ToJSON())
			})

			assertFailure("Stopping fetcher", 1)
		})

		Context("when an unauthorized response is received", func() {
			JustBeforeEach(func() {
				httpClient.LastRequest().RespondWithStatus(http.StatusUnauthorized)
			})

			assertFailure("HTTP request received unauthorized response code", 1)
		})

		Context("when the HTTP request returns a non-200 response", func() {
			JustBeforeEach(func() {
				httpClient.LastRequest().RespondWithStatus(http.StatusNotFound)
			})

			assertFailure("HTTP request received non-200 response (404)", 1)
		})

		Context("when the HTTP request fails with an error", func() {
			JustBeforeEach(func() {
				httpClient.LastRequest().RespondWithError(errors.New(":("))
			})

			assertFailure("HTTP request failed with error", 1)
		})

		Context("when a broken body is received", func() {
			JustBeforeEach(func() {
				response := &http.Response{
					Status:     "StatusOK (200)",
					StatusCode: http.StatusOK,

					ContentLength: 17,
					Body:          &brokenReader{},
				}

				httpClient.LastRequest().Callback(response, nil)
			})

			assertFailure("Failed to read HTTP response body", 1)
		})

		Context("when a malformed response is received", func() {
			JustBeforeEach(func() {
				httpClient.LastRequest().Succeed([]byte("ß"))
			})

			assertFailure("Failed to parse HTTP response body JSON", 1)
		})
	})
})
