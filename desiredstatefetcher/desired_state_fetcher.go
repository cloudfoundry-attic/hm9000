package desiredstatefetcher

import (
	"fmt"
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/httpclient"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"net/http"
)

type DesiredStateFetcherResult struct {
	Success    bool
	Message    string
	Error      error
	NumResults int
}

const initialBulkToken = "{}"

type DesiredStateFetcher struct {
	config       config.Config
	messageBus   cfmessagebus.MessageBus
	httpClient   httpclient.HttpClient
	store        store.Store
	timeProvider timeprovider.TimeProvider
}

func New(config config.Config,
	messageBus cfmessagebus.MessageBus,
	store store.Store,
	httpClient httpclient.HttpClient,
	timeProvider timeprovider.TimeProvider) *DesiredStateFetcher {

	return &DesiredStateFetcher{
		config:       config,
		messageBus:   messageBus,
		httpClient:   httpClient,
		store:        store,
		timeProvider: timeProvider,
	}
}

func (fetcher *DesiredStateFetcher) Fetch(resultChan chan DesiredStateFetcherResult) {
	err := fetcher.messageBus.Request(fetcher.config.CCAuthMessageBusSubject, []byte{}, func(response []byte) {
		authInfo, err := models.NewBasicAuthInfoFromJSON(response)
		if err != nil {
			resultChan <- DesiredStateFetcherResult{Message: "Failed to parse authentication info from JSON", Error: err}
			return
		}

		fetcher.fetchBatch(authInfo.Encode(), initialBulkToken, 0, resultChan)
	})
	if err != nil {
		resultChan <- DesiredStateFetcherResult{Message: "Failed to request auth info", Error: err}
	}
}

func (fetcher *DesiredStateFetcher) fetchBatch(authorization string, token string, numResults int, resultChan chan DesiredStateFetcherResult) {
	req, err := http.NewRequest("GET", fetcher.bulkURL(fetcher.config.DesiredStateBatchSize, token), nil)

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

		response, err := NewDesiredStateServerResponse(body)
		if err != nil {
			resultChan <- DesiredStateFetcherResult{Message: "Failed to parse HTTP response body JSON", Error: err}
			return
		}
		if len(response.Results) == 0 {
			fetcher.store.BumpDesiredFreshness(fetcher.timeProvider.Time())
			resultChan <- DesiredStateFetcherResult{Success: true, NumResults: numResults}
			return
		}

		err = fetcher.saveToStore(response)
		if err != nil {
			resultChan <- DesiredStateFetcherResult{Message: "Failed to store desired state in store", Error: err}
			return
		}
		fetcher.fetchBatch(authorization, response.BulkTokenRepresentation(), numResults+len(response.Results), resultChan)
	})
}

func (fetcher *DesiredStateFetcher) bulkURL(batchSize int, bulkToken string) string {
	return fmt.Sprintf("%s/bulk/apps?batch_size=%d&bulk_token=%s", fetcher.config.CCBaseURL, batchSize, bulkToken)
}

func (fetcher *DesiredStateFetcher) saveToStore(response DesiredStateServerResponse) error {
	desiredStates := make([]models.DesiredAppState, len(response.Results))
	i := 0
	for _, desiredState := range response.Results {
		desiredStates[i] = desiredState
		i++
	}
	return fetcher.store.SaveDesiredState(desiredStates)
}
