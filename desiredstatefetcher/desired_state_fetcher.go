package desiredstatefetcher

import (
	"fmt"
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/bel_air"
	"github.com/cloudfoundry/hm9000/helpers/http_client"
	"github.com/cloudfoundry/hm9000/helpers/time_provider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"net/http"
)

type DesiredStateFetcherResult struct {
	Success bool
	Message string
	Error   error
}

const initialBulkToken = "{}"

type desiredStateFetcher struct {
	messageBus   cfmessagebus.MessageBus
	httpClient   http_client.HttpClient
	store        store.Store
	freshPrince  bel_air.FreshPrince
	timeProvider time_provider.TimeProvider
	ccBaseURL    string
	batchSize    int
}

func NewDesiredStateFetcher(messageBus cfmessagebus.MessageBus,
	store store.Store,
	httpClient http_client.HttpClient,
	freshPrince bel_air.FreshPrince,
	timeProvider time_provider.TimeProvider,
	ccBaseURL string,
	batchSize int) *desiredStateFetcher {

	return &desiredStateFetcher{
		messageBus:   messageBus,
		httpClient:   httpClient,
		store:        store,
		freshPrince:  freshPrince,
		timeProvider: timeProvider,
		ccBaseURL:    ccBaseURL,
		batchSize:    batchSize,
	}
}

func (fetcher *desiredStateFetcher) Fetch(resultChan chan DesiredStateFetcherResult) {
	fetcher.messageBus.Request("cloudcontroller.bulk.credentials.default", []byte{}, func(response []byte) {
		authInfo, err := models.NewBasicAuthInfoFromJSON(response)
		if err != nil {
			resultChan <- DesiredStateFetcherResult{Message: "Failed to parse authentication info from JSON", Error: err}
			return
		}

		fetcher.fetchBatch(authInfo.Encode(), initialBulkToken, resultChan)
	})
}

func (fetcher *desiredStateFetcher) fetchBatch(authorization string, token string, resultChan chan DesiredStateFetcherResult) {
	req, err := http.NewRequest("GET", fetcher.bulkURL(fetcher.batchSize, token), nil)

	if err != nil {
		resultChan <- DesiredStateFetcherResult{Message: "Failed to generate URL request", Error: err}
		return
	}

	req.Header.Add("Authorization", authorization)

	fetcher.httpClient.Do(req, func(resp *http.Response, err error) {
		if err != nil {
			resultChan <- DesiredStateFetcherResult{Message: "HTTP request failed with error", Error: err}
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			resultChan <- DesiredStateFetcherResult{Message: "HTTP request received unauthorized response code", Error: fmt.Errorf("Unauthorized")}
			return
		}

		if resp.StatusCode != http.StatusOK {
			resultChan <- DesiredStateFetcherResult{Message: fmt.Sprintf("HTTP request received non-200 response (%d)", resp.StatusCode), Error: fmt.Errorf("Invalid response code")}
			return
		}

		body := make([]byte, resp.ContentLength)
		_, err = resp.Body.Read(body)

		if err != nil {
			resultChan <- DesiredStateFetcherResult{Message: "Failed to read HTTP response body", Error: err}
			return
		}

		desiredState, err := NewDesiredStateServerResponse(body)
		if err != nil {
			resultChan <- DesiredStateFetcherResult{Message: "Failed to parse HTTP response body JSON", Error: err}
			return
		}
		if len(desiredState.Results) == 0 {
			fetcher.freshPrince.Bump(config.DESIRED_FRESHNESS_KEY, config.DESIRED_FRESHNESS_TTL, fetcher.timeProvider.Time())
			resultChan <- DesiredStateFetcherResult{Success: true}
			return
		}

		err = fetcher.pushToStore(desiredState)
		if err != nil {
			resultChan <- DesiredStateFetcherResult{Message: "Failed to store desired state in store", Error: err}
			return
		}

		fetcher.fetchBatch(authorization, desiredState.BulkTokenRepresentation(), resultChan)
	})
}

func (fetcher *desiredStateFetcher) bulkURL(batchSize int, bulkToken string) string {
	return fmt.Sprintf("%s/bulk/apps?batch_size=%d&bulk_token=%s", fetcher.ccBaseURL, batchSize, bulkToken)
}

func (fetcher *desiredStateFetcher) pushToStore(desiredState desiredStateServerResponse) error {
	for _, desiredAppState := range desiredState.Results {
		key := "/desired/" + desiredAppState.AppGuid + "-" + desiredAppState.AppVersion
		err := fetcher.store.Set(key, desiredAppState.ToJson(), config.DESIRED_STATE_TTL)
		if err != nil {
			return err
		}
	}

	return nil
}
