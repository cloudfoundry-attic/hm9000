package desiredstateserver

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"
	"fmt"
	"github.com/cloudfoundry/go_cfmessagebus/mock_cfmessagebus"
	. "github.com/cloudfoundry/hm9000/models"

	"net/http"
	"testing"
	"time"
)

type UserCountResponse struct {
	Counts struct {
		User int `json:"user"`
	} `json:"counts"`
}

func TestDesiredStateServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Desired State Server Suite")
}

var server *DesiredStateServer
var didRunGlobalBeforeEach bool
var fakeMessageBus *mock_cfmessagebus.MockMessageBus

var _ = BeforeEach(func() {
	if !didRunGlobalBeforeEach {
		fakeMessageBus = mock_cfmessagebus.NewMockMessageBus()
		server = NewDesiredStateServer(fakeMessageBus)
		go server.SpinUp(6000)
		didRunGlobalBeforeEach = true
	}

	server.Reset()
})

var _ = Describe("making requests", func() {
	var serverURL = "http://localhost:6000"

	Describe("/bulk/counts", func() {
		var response UserCountResponse
		BeforeEach(func(done Done) {
			url := fmt.Sprintf("%s/bulk/counts?model=user", serverURL)
			resp, err := http.Get(url)
			Ω(err).ShouldNot(HaveOccured())
			Ω(resp.StatusCode).Should(Equal(http.StatusOK))

			body := make([]byte, resp.ContentLength)
			_, err = resp.Body.Read(body)
			Ω(err).ShouldNot(HaveOccured())

			err = json.Unmarshal(body, &response)

			done <- true
		}, 2)

		It("Returns a user count", func() {
			Ω(response.Counts.User).Should(Equal(17))
		})
	})

	Describe("/bulk/apps", func() {
		var (
			batchSize       uint
			bulkTokenAsJson string
			authorization   string
			resp            *http.Response
		)

		BeforeEach(func() {
			batchSize = 10
			bulkTokenAsJson = "{}"
			authorization = "Basic bWNhdDp0ZXN0aW5n"
		})

		JustBeforeEach(func(done Done) {
			url := fmt.Sprintf("%s/bulk/apps?batch_size=%d&bulk_token=%s", serverURL, batchSize, bulkTokenAsJson)
			req, err := http.NewRequest("GET", url, nil)
			Ω(err).ShouldNot(HaveOccured())
			if authorization != "" {
				req.Header.Add("Authorization", authorization)
			}
			client := &http.Client{}
			resp, err = client.Do(req)

			Ω(err).ShouldNot(HaveOccured())
			done <- true
		}, 2)

		AfterEach(func() {
			resp.Body.Close()
		})

		Context("without credentials", func() {
			BeforeEach(func() {
				authorization = ""
			})

			It("It should return a 401 error", func() {
				Ω(resp.StatusCode).Should(Equal(http.StatusUnauthorized))
				Ω(resp.ContentLength).Should(BeNumerically("==", 0))
			})
		})

		Context("with incorrect credentials", func() {
			BeforeEach(func() {
				authorization = "Basic BLABLABLAINCORRECt"
			})

			It("It should return a 401 error", func() {
				Ω(resp.StatusCode).Should(Equal(http.StatusUnauthorized))
				Ω(resp.ContentLength).Should(BeNumerically("==", 0))
			})
		})

		Context("with correct authorization", func() {
			var (
				response desiredStateServerResponse
				app1     DesiredAppState
				app2     DesiredAppState
				app3     DesiredAppState
			)

			BeforeEach(func() {
				response = desiredStateServerResponse{}
				app1 = DesiredAppState{
					AppGuid:           "abc",
					AppVersion:        "123",
					NumberOfInstances: 2,
					Memory:            1024,
					State:             AppStateStarted,
					PackageState:      AppPackageStateStaged,
					UpdatedAt:         time.Unix(1377886048, 0),
				}
				app2 = DesiredAppState{
					AppGuid:           "def",
					AppVersion:        "456",
					NumberOfInstances: 3,
					Memory:            1024,
					State:             AppStateStopped,
					PackageState:      AppPackageStatePending,
					UpdatedAt:         time.Unix(1377886048, 0),
				}
				app3 = DesiredAppState{
					AppGuid:           "cba",
					AppVersion:        "789",
					NumberOfInstances: 2,
					Memory:            1024,
					State:             AppStateStopped,
					PackageState:      AppPackageStatePending,
					UpdatedAt:         time.Unix(1377886048, 0),
				}
			})

			JustBeforeEach(func() {
				Ω(resp.StatusCode).Should(Equal(http.StatusOK))

				body := make([]byte, resp.ContentLength)
				_, err := resp.Body.Read(body)
				Ω(err).ShouldNot(HaveOccured())

				err = json.Unmarshal(body, &response)
			})

			Context("when there are apps", func() {
				BeforeEach(func() {
					server.SetDesiredState([]DesiredAppState{app1, app2})
				})

				It("JSON encodes and returns the desired state", func() {
					Ω(response.Results).Should(HaveLen(2))
					Ω(response.Results["abc"]).Should(EqualDesiredState(app1))
					Ω(response.Results["def"]).Should(EqualDesiredState(app2))
					Ω(response.BulkToken.Id).Should(Equal(2))
				})
			})

			Context("when there are no apps", func() {
				BeforeEach(func() {
					server.SetDesiredState([]DesiredAppState{})
				})

				It("returns an empty list of apps", func() {
					Ω(response.Results).Should(Equal(map[string]DesiredAppState{}))
					Ω(response.BulkToken.Id).Should(Equal(0))
				})
			})

			Context("when there are more apps than the batch size", func() {
				BeforeEach(func() {
					server.SetDesiredState([]DesiredAppState{app1, app2, app3})
					batchSize = 2
				})

				It("returns a list only the length of the batch size", func() {
					Ω(response.Results).Should(HaveLen(2))
					Ω(response.Results["abc"]).Should(EqualDesiredState(app1))
					Ω(response.Results["def"]).Should(EqualDesiredState(app2))
					Ω(response.BulkToken.Id).Should(Equal(2))
				})

				It("does not increment the NumberOfCompleteFetches counter", func() {
					Ω(server.NumberOfCompleteFetches).Should(Equal(0))
				})

				Context("when fetching the next batch", func() {
					BeforeEach(func() {
						batchSize = 2
						bulkTokenAsJson = `{"id":2}`
					})

					It("returns a list only the length of the batch size", func() {
						Ω(response.Results).Should(HaveLen(1))
						Ω(response.Results["cba"]).Should(EqualDesiredState(app3))
						Ω(response.BulkToken.Id).Should(Equal(3))
					})

					It("does not increment the NumberOfCompleteFetches counter", func() {
						Ω(server.NumberOfCompleteFetches).Should(Equal(0))
					})
				})

				Context("when fetching the 'last' batch", func() {
					BeforeEach(func() {
						batchSize = 2
						bulkTokenAsJson = `{"id":3}`
					})

					It("returns a list only the length of the batch size", func() {
						Ω(response.Results).Should(BeEmpty())
						Ω(response.BulkToken.Id).Should(Equal(3))
					})

					It("increments the NumberOfCompleteFetches counter", func() {
						Ω(server.NumberOfCompleteFetches).Should(Equal(1))
					})
				})
			})
		})
	})
})

var _ = Describe("fetch credetials", func() {
	It("should respond to 'cloudcontroller.bulk.credentials.default' with a username and password", func(done Done) {
		response := make(chan string, 0)
		err := fakeMessageBus.Request("cloudcontroller.bulk.credentials.default", []byte{}, func(r []byte) {
			response <- string(r)
		})

		Ω(err).ShouldNot(HaveOccured())
		Ω(<-response).Should(Equal(`{"user":"mcat","password":"testing"}`))

		done <- true
	}, 2)
})
