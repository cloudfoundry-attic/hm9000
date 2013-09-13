package desiredstatepoller

import (
	"fmt"
	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/test_helpers/app"
	"github.com/cloudfoundry/hm9000/test_helpers/fake_bel_air"
	"github.com/cloudfoundry/hm9000/test_helpers/fake_http_client"
	"github.com/cloudfoundry/hm9000/test_helpers/fake_time_provider"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
)

var _ = Describe("DesiredStatePoller", func() {
	var (
		fakeMessageBus *fake_cfmessagebus.FakeMessageBus
		poller         *desiredStatePoller
		httpClient     *fake_http_client.FakeHttpClient
		timeProvider   *fake_time_provider.FakeTimeProvider
		freshPrince    *fake_bel_air.FakeFreshPrince
	)

	BeforeEach(func() {
		timeProvider = &fake_time_provider.FakeTimeProvider{
			TimeToProvide: time.Now(),
		}

		fakeMessageBus = fake_cfmessagebus.NewFakeMessageBus()
		httpClient = fake_http_client.NewFakeHttpClient()
		freshPrince = &fake_bel_air.FakeFreshPrince{}

		poller = NewDesiredStatePoller(fakeMessageBus, etcdStore, httpClient, freshPrince, timeProvider, desiredStateServerBaseUrl, config.DESIRED_STATE_POLLING_BATCH_SIZE)
		poller.Poll()
	})

	Describe("Authentication", func() {
		It("should request the CC credentials over NATS", func() {
			Ω(fakeMessageBus.Requests).Should(HaveKey(authNatsSubject))
			Ω(fakeMessageBus.Requests[authNatsSubject]).Should(HaveLen(1))
			Ω(fakeMessageBus.Requests[authNatsSubject][0].Message).Should(BeEmpty())
		})

		Context("when we've received the authentication information", func() {
			var request *fake_http_client.Request

			BeforeEach(func() {
				fakeMessageBus.Requests[authNatsSubject][0].Callback([]byte(`{"user":"mcat","password":"testing"}`))
				request = httpClient.LastRequest()
			})

			It("should make the correct request", func() {
				Ω(httpClient.Requests).Should(HaveLen(1))

				Ω(request.URL.String()).Should(ContainSubstring(desiredStateServerBaseUrl))
				Ω(request.URL.Path).Should(ContainSubstring("/bulk/apps"))

				expectedAuth := models.BasicAuthInfo{
					User:     "mcat",
					Password: "testing",
				}.Encode()

				Ω(request.Header.Get("Authorization")).Should(Equal(expectedAuth))
			})

			Context("when the authentication fails", func() {
				BeforeEach(func() {
					request.RespondWithStatus(http.StatusUnauthorized)
				})

				It("should fetch the authentication again when it polls next", func() {
					poller.Poll()
					Ω(fakeMessageBus.Requests[authNatsSubject]).Should(HaveLen(2))
				})
			})

			Context("when the authentication succeeds", func() {
				BeforeEach(func() {
					request.RespondWithStatus(http.StatusOK)
				})

				It("should not fetch the authentication again when it polls next", func() {
					httpClient.Reset()
					poller.Poll()
					Ω(fakeMessageBus.Requests[authNatsSubject]).Should(HaveLen(1))
					Ω(httpClient.Requests).Should(HaveLen(1))
				})
			})
		})

		Context("when the authentication information was corrupted", func() {
			BeforeEach(func() {
				fakeMessageBus.Requests[authNatsSubject][0].Callback([]byte(`{`))
			})

			It("should not make any requests", func() {
				Ω(httpClient.Requests).Should(HaveLen(0))
			})

			It("should fetch the authentication again when it polls next", func() {
				poller.Poll()
				Ω(fakeMessageBus.Requests[authNatsSubject]).Should(HaveLen(2))
			})
		})

		Context("when the authentication information fails to arrive", func() {
			It("should not make any requests", func() {
				Ω(httpClient.Requests).Should(HaveLen(0))
			})

			It("should fetch the authentication again when it polls next", func() {
				poller.Poll()
				Ω(fakeMessageBus.Requests[authNatsSubject]).Should(HaveLen(2))
			})
		})
	})

	Describe("Fetching batches", func() {
		var response desiredStateServerResponse

		BeforeEach(func() {
			fakeMessageBus.Requests[authNatsSubject][0].Callback([]byte(`{"user":"mcat","password":"testing"}`))
		})

		It("should request a batch size with an empty bulk token", func() {
			query := httpClient.LastRequest().URL.Query()
			Ω(query.Get("batch_size")).Should(Equal(fmt.Sprintf("%d", config.DESIRED_STATE_POLLING_BATCH_SIZE)))
			Ω(query.Get("bulk_token")).Should(Equal("{}"))
		})

		Context("when a response with desired state is received", func() {
			var (
				a1 app.App
				a2 app.App
			)

			BeforeEach(func() {
				a1 = app.NewApp()
				a2 = app.NewApp()

				response = desiredStateServerResponse{
					Results: map[string]models.DesiredAppState{
						a1.AppGuid: a1.DesiredState(0),
						a2.AppGuid: a2.DesiredState(0),
					},
					BulkToken: bulkToken{
						Id: 5,
					},
				}

				httpClient.LastRequest().Succeed(response.ToJson())
			})

			It("should store the desired states", func() {
				node, err := etcdStore.Get("/desired/" + a1.AppGuid + "-" + a1.AppVersion)
				Ω(err).ShouldNot(HaveOccured())

				Ω(node.TTL).Should(BeNumerically("==", config.DESIRED_STATE_TTL-1))

				Ω(node.Value).Should(Equal(a1.DesiredState(0).ToJson()))

				node, err = etcdStore.Get("/desired/" + a2.AppGuid + "-" + a2.AppVersion)
				Ω(err).ShouldNot(HaveOccured())

				Ω(node.TTL).Should(BeNumerically("==", config.DESIRED_STATE_TTL-1))

				Ω(node.Value).Should(Equal(a2.DesiredState(0).ToJson()))
			})

			It("should request the next batch", func() {
				Ω(httpClient.Requests).Should(HaveLen(2))
				Ω(httpClient.LastRequest().URL.Query().Get("bulk_token")).Should(Equal(response.BulkTokenRepresentation()))
			})

			It("should not bump the freshness yet", func() {
				Ω(freshPrince.Key).Should(BeZero())
			})
		})

		Context("when a malformed response is received", func() {
			BeforeEach(func() {
				httpClient.LastRequest().Succeed([]byte("ß"))
			})

			It("should stop requesting batches", func() {
				Ω(httpClient.Requests).Should(HaveLen(1))
			})

			It("should not bump the freshness", func() {
				Ω(freshPrince.Key).Should(BeZero())
			})
		})

		Context("when an unauthorized response is received", func() {
			BeforeEach(func() {
				httpClient.LastRequest().RespondWithStatus(http.StatusUnauthorized)
			})

			It("should stop requesting batches", func() {
				Ω(httpClient.Requests).Should(HaveLen(1))
			})

			It("should not bump the freshness", func() {
				Ω(freshPrince.Key).Should(BeZero())
			})
		})

		Context("when an empty response is received", func() {
			BeforeEach(func() {
				response = desiredStateServerResponse{
					Results: map[string]models.DesiredAppState{},
					BulkToken: bulkToken{
						Id: 17,
					},
				}

				httpClient.LastRequest().Succeed(response.ToJson())
			})

			It("should stop requesting batches", func() {
				Ω(httpClient.Requests).Should(HaveLen(1))
			})

			It("should bump the freshness", func() {
				Ω(freshPrince.Key).Should(Equal(config.DESIRED_FRESHNESS_KEY))
				Ω(freshPrince.Timestamp).Should(Equal(timeProvider.Time()))
				Ω(freshPrince.TTL).Should(BeNumerically("==", config.DESIRED_FRESHNESS_TTL))
			})
		})
	})
})
