package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cloudfoundry/gunk/timeprovider/faketimeprovider"
	"github.com/cloudfoundry/hm9000/apiserver/handlers"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func decodeBulkResponse(response string) (bulkAppResp map[string]AppResponse) {
	err := json.Unmarshal([]byte(response), &bulkAppResp)
	Expect(err).NotTo(HaveOccurred())
	return
}

type AppResponse struct {
	AppGuid    string `json:"droplet"`
	AppVersion string `json:"version"`

	Desired            models.DesiredAppState     `json:"desired"`
	InstanceHeartbeats []models.InstanceHeartbeat `json:"instance_heartbeats"`
	CrashCounts        []models.CrashCount        `json:"crash_counts"`
}

type HandlerConf struct {
	StoreAdapter *fakestoreadapter.FakeStoreAdapter
	TimeProvider *faketimeprovider.FakeTimeProvider
	Logger       logger.Logger
	MaxInFlight  int
}

func defaultConf() HandlerConf {
	return HandlerConf{
		StoreAdapter: fakestoreadapter.New(),
		TimeProvider: &faketimeprovider.FakeTimeProvider{
			TimeToProvide: time.Unix(100, 0),
		},
		Logger: fakelogger.NewFakeLogger(),
	}
}

func makeHandlerAndStore(conf HandlerConf) (http.Handler, store.Store, error) {
	config, _ := config.DefaultConfig()

	store := store.NewStore(config, conf.StoreAdapter, fakelogger.NewFakeLogger())

	handler, err := handlers.New(conf.Logger, store, conf.TimeProvider)
	return handler, store, err
}

func decodeResponse(response string) (appResp AppResponse) {
	err := json.Unmarshal([]byte(response), &appResp)
	Ω(err).ShouldNot(HaveOccurred())
	return appResp
}

func freshenTheStore(store store.Store) {
	store.BumpDesiredFreshness(time.Unix(0, 0))
	store.BumpActualFreshness(time.Unix(0, 0))
}

var _ = Describe("BulkAppState", func() {
	Context("when the store has an unexpected error", func() {
		It("should return an empty hash", func() {
			conf := defaultConf()
			conf.StoreAdapter.GetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("desired", fmt.Errorf("No desired state for you!"))

			handler, store, err := makeHandlerAndStore(conf)
			Expect(err).ToNot(HaveOccurred())
			freshenTheStore(store)

			request, _ := http.NewRequest("POST", "/bulk_app_state", bytes.NewBufferString(`[{"droplet":"something","version":"whatever"}]`))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			Expect(response.Body.String()).To(Equal("{}"))
		})
	})

	Context("when the store is not fresh", func() {
		It("returns an empty hash", func() {
			conf := defaultConf()
			app := appfixture.NewAppFixture()

			handler, store, err := makeHandlerAndStore(conf)
			Expect(err).ToNot(HaveOccurred())

			crashCount := models.CrashCount{
				AppGuid:       app.AppGuid,
				AppVersion:    app.AppVersion,
				InstanceIndex: 1,
				CrashCount:    2,
			}
			store.SyncDesiredState(app.DesiredState(3))
			store.SyncHeartbeats(app.Heartbeat(3))
			store.SaveCrashCounts(crashCount)

			request_body := fmt.Sprintf(`[{"droplet":"%s","version":"%s"}]`, app.AppGuid, app.AppVersion)
			request, _ := http.NewRequest("POST", "/bulk_app_state", bytes.NewBufferString(request_body))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			Expect(response.Body.String()).To(Equal("{}"))
		})
	})

	Context("when the store is fresh", func() {
		It("returns an empty hash when invalid request json is provided", func() {
			conf := defaultConf()
			app := appfixture.NewAppFixture()

			handler, store, err := makeHandlerAndStore(conf)
			Expect(err).ToNot(HaveOccurred())

			crashCount := models.CrashCount{
				AppGuid:       app.AppGuid,
				AppVersion:    app.AppVersion,
				InstanceIndex: 1,
				CrashCount:    2,
			}
			store.SyncDesiredState(app.DesiredState(3))
			store.SyncHeartbeats(app.Heartbeat(3))
			store.SaveCrashCounts(crashCount)
			freshenTheStore(store)

			request, _ := http.NewRequest("POST", "/bulk_app_state", bytes.NewBufferString("asdf{}"))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			Expect(response.Body.String()).To(Equal("{}"))
		})

		Context("when the app query parameters do not correspond to an existing app", func() {
			It("returns empty hash", func() {
				conf := defaultConf()
				handler, store, err := makeHandlerAndStore(conf)
				Expect(err).ToNot(HaveOccurred())
				freshenTheStore(store)

				request, _ := http.NewRequest("POST", "/bulk_app_state", bytes.NewBufferString(`[{"droplet":"elephant","version":"pink-flamingo"}]`))
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)

				Expect(response.Body.String()).To(Equal("{}"))
			})
		})

		Context("when the app query parameters correspond to an existing app", func() {
			It("should return the actual instances and crashes of the app", func() {
				conf := defaultConf()
				app := appfixture.NewAppFixture()

				handler, store, err := makeHandlerAndStore(conf)
				Expect(err).ToNot(HaveOccurred())

				crashCount := models.CrashCount{
					AppGuid:       app.AppGuid,
					AppVersion:    app.AppVersion,
					InstanceIndex: 1,
					CrashCount:    2,
				}
				store.SyncDesiredState(app.DesiredState(3))
				store.SyncHeartbeats(app.Heartbeat(3))
				store.SaveCrashCounts(crashCount)
				freshenTheStore(store)

				request_body := fmt.Sprintf(`[{"droplet":"%s","version":"%s"}]`, app.AppGuid, app.AppVersion)
				request, _ := http.NewRequest("POST", "/bulk_app_state", bytes.NewBufferString(request_body))
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)

				expectedInstanceHeartbeats := []models.InstanceHeartbeat{
					app.InstanceAtIndex(0).Heartbeat(),
					app.InstanceAtIndex(1).Heartbeat(),
					app.InstanceAtIndex(2).Heartbeat(),
				}
				expectedApp := AppResponse{
					AppGuid:            app.AppGuid,
					AppVersion:         app.AppVersion,
					Desired:            app.DesiredState(3),
					InstanceHeartbeats: expectedInstanceHeartbeats,
					CrashCounts:        []models.CrashCount{crashCount},
				}

				decodedResponse := decodeBulkResponse(response.Body.String())
				Expect(decodedResponse).To(HaveLen(1))
				Expect(decodedResponse).To(HaveKey(expectedApp.AppGuid))
				receivedApp := decodedResponse[expectedApp.AppGuid]
				Expect(receivedApp.AppGuid).To(Equal(expectedApp.AppGuid))
				Expect(receivedApp.AppVersion).To(Equal(expectedApp.AppVersion))
				Expect(receivedApp.Desired).To(Equal(expectedApp.Desired))
				Expect(receivedApp.InstanceHeartbeats).To(ConsistOf(expectedApp.InstanceHeartbeats))
				Expect(receivedApp.CrashCounts).To(ConsistOf(expectedApp.CrashCounts))
			})
		})

		Context("when some of the apps are not found", func() {
			It("responds with the apps that are present", func() {
				conf := defaultConf()
				app := appfixture.NewAppFixture()

				handler, store, err := makeHandlerAndStore(conf)
				Expect(err).ToNot(HaveOccurred())
				freshenTheStore(store)

				crashCount := models.CrashCount{
					AppGuid:       app.AppGuid,
					AppVersion:    app.AppVersion,
					InstanceIndex: 1,
					CrashCount:    2,
				}
				store.SyncDesiredState(app.DesiredState(3))
				store.SyncHeartbeats(app.Heartbeat(3))
				store.SaveCrashCounts(crashCount)

				requestBody := fmt.Sprintf(`[{"droplet":"%s","version":"%s"},{"droplet":"jam-sandwich","version":"123"}]`, app.AppGuid, app.AppVersion)
				request, _ := http.NewRequest("POST", "/bulk_app_state", bytes.NewBufferString(requestBody))
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)

				expectedInstanceHeartbeats := []models.InstanceHeartbeat{
					app.InstanceAtIndex(0).Heartbeat(),
					app.InstanceAtIndex(1).Heartbeat(),
					app.InstanceAtIndex(2).Heartbeat(),
				}
				expectedApp := AppResponse{
					AppGuid:            app.AppGuid,
					AppVersion:         app.AppVersion,
					Desired:            app.DesiredState(3),
					InstanceHeartbeats: expectedInstanceHeartbeats,
					CrashCounts:        []models.CrashCount{crashCount},
				}

				decodedResponse := decodeBulkResponse(response.Body.String())
				Expect(decodedResponse).To(HaveLen(1))
				Expect(decodedResponse).To(HaveKey(expectedApp.AppGuid))
				receivedApp := decodedResponse[expectedApp.AppGuid]
				Expect(receivedApp.AppGuid).To(Equal(expectedApp.AppGuid))
				Expect(receivedApp.AppVersion).To(Equal(expectedApp.AppVersion))
				Expect(receivedApp.Desired).To(Equal(expectedApp.Desired))
				Expect(receivedApp.InstanceHeartbeats).To(ConsistOf(expectedApp.InstanceHeartbeats))
				Expect(receivedApp.CrashCounts).To(ConsistOf(expectedApp.CrashCounts))
			})
		})
	})
})
