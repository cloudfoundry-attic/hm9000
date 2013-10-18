package apiserver_test

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/apiserver"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestoreadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"net/http"
	"time"
)

var didSetup bool

const port = 8090

var _ = Describe("Apiserver", func() {
	//create the server
	//(eventually) passing it whatever depenencies it needs
	//start the server

	var store storepackage.Store
	var storeAdapter *fakestoreadapter.FakeStoreAdapter
	var timeProvider *faketimeprovider.FakeTimeProvider

	conf, _ := config.DefaultConfig()

	makeGetRequest := func(url string) (statusCode int, body string) {
		resp, err := http.Get(url)
		Ω(err).ShouldNot(HaveOccured())

		defer resp.Body.Close()

		statusCode = resp.StatusCode

		bodyAsBytes, err := ioutil.ReadAll(resp.Body)
		Ω(err).ShouldNot(HaveOccured())
		body = string(bodyAsBytes)
		return
	}

	BeforeEach(func() {
		if !didSetup {
			storeAdapter = fakestoreadapter.New()
			store = storepackage.NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
			timeProvider = &faketimeprovider.FakeTimeProvider{
				TimeToProvide: time.Unix(100, 0),
			}
			server := apiserver.New(port, store, timeProvider, conf, fakelogger.NewFakeLogger())
			go server.Start()

			Eventually(func() int {
				statusCode, _ := makeGetRequest(fmt.Sprintf("http://magnet:orangutan4sale@localhost:%d/app", port))
				return statusCode
			}, 2, 0.01).Should(Equal(http.StatusBadRequest))

			didSetup = true
		}
		timeProvider.TimeToProvide = time.Unix(100, 0)
		storeAdapter.Reset()
	})

	Context("when serving /app", func() {
		Context("when there are no query parameters", func() {
			It("should return a 400 with no data", func() {
				statusCode, body := makeGetRequest(fmt.Sprintf("http://magnet:orangutan4sale@localhost:%d/app", port))
				Ω(statusCode).Should(Equal(http.StatusBadRequest))
				Ω(body).Should(BeEmpty())
			})
		})

		Context("when the app query parameters are present", func() {
			var app appfixture.AppFixture
			var expectedApp *models.App
			var validRequestURL string

			BeforeEach(func() {
				app = appfixture.NewAppFixture()
				instanceHeartbeats := []models.InstanceHeartbeat{
					app.InstanceAtIndex(0).Heartbeat(),
					app.InstanceAtIndex(1).Heartbeat(),
					app.InstanceAtIndex(2).Heartbeat(),
				}
				crashCount := models.CrashCount{
					AppGuid:       app.AppGuid,
					AppVersion:    app.AppVersion,
					InstanceIndex: 1,
					CrashCount:    2,
				}
				expectedApp = models.NewApp(
					app.AppGuid,
					app.AppVersion,
					app.DesiredState(3),
					instanceHeartbeats,
					map[int]models.CrashCount{1: crashCount},
				)

				store.SaveDesiredState(app.DesiredState(3))
				store.SaveActualState(instanceHeartbeats...)
				store.SaveCrashCounts(crashCount)
				validRequestURL = fmt.Sprintf("http://magnet:orangutan4sale@localhost:%d/app?app-guid=%s&app-version=%s", port, app.AppGuid, app.AppVersion)
			})

			Context("when the store is fresh", func() {
				BeforeEach(func() {
					store.BumpDesiredFreshness(time.Unix(0, 0))
					store.BumpActualFreshness(time.Unix(0, 0))
				})

				Context("when the app query parameters do not correspond to an existing app", func() {
					It("should return a 404 not found response", func() {
						statusCode, body := makeGetRequest(
							fmt.Sprintf("http://magnet:orangutan4sale@localhost:%d/app?app-guid=elephant&app-version=pink-flamingo", port),
						)
						Ω(statusCode).Should(Equal(http.StatusNotFound))
						Ω(body).Should(BeEmpty())
					})
				})

				Context("when the app query parameters correspond to an existing app", func() {
					It("should return the actual instances and crashes of the app", func() {
						statusCode, body := makeGetRequest(validRequestURL)
						Ω(statusCode).Should(Equal(http.StatusOK))

						Ω(body).Should(Equal(string(expectedApp.ToJSON())))
					})
				})

				Context("when the auth credentials are wrong", func() {
					It("should 401", func() {
						statusCode, body := makeGetRequest(fmt.Sprintf("http://solenoid:chimpanzee8toad@localhost:%d/app?app-guid=%s&app-version=%s", port, app.AppGuid, app.AppVersion))
						Ω(statusCode).Should(Equal(http.StatusUnauthorized))
						Ω(body).Should(BeEmpty())
					})
				})

				Context("when the auth credentials are missing", func() {
					It("should 401", func() {
						statusCode, body := makeGetRequest(fmt.Sprintf("http://localhost:%d/app?app-guid=%s&app-version=%s", port, app.AppGuid, app.AppVersion))
						Ω(statusCode).Should(Equal(http.StatusUnauthorized))
						Ω(body).Should(BeEmpty())
					})
				})

				Context("when something else goes wrong with the store", func() {
					BeforeEach(func() {
						storeAdapter.GetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("desired", fmt.Errorf("No desired state for you!"))
					})

					It("should 500", func() {
						statusCode, body := makeGetRequest(validRequestURL)
						Ω(statusCode).Should(Equal(http.StatusInternalServerError))
						Ω(body).Should(BeEmpty())
					})
				})
			})

			Context("when the store is not fresh", func() {
				It("should return a 404", func() {
					statusCode, body := makeGetRequest(validRequestURL)
					Ω(statusCode).Should(Equal(http.StatusNotFound))
					Ω(body).Should(BeEmpty())
				})
			})
		})
	})
})
