package desiredstatefetcher

import (
	"fmt"
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/freshnessmanager"
	"github.com/cloudfoundry/hm9000/helpers/httpclient"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
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
	config           config.Config
	messageBus       cfmessagebus.MessageBus
	httpClient       httpclient.HttpClient
	storeAdapter     storeadapter.StoreAdapter
	freshnessManager freshnessmanager.FreshnessManager
	timeProvider     timeprovider.TimeProvider
}

func New(config config.Config,
	messageBus cfmessagebus.MessageBus,
	storeAdapter storeadapter.StoreAdapter,
	httpClient httpclient.HttpClient,
	freshnessManager freshnessmanager.FreshnessManager,
	timeProvider timeprovider.TimeProvider) *DesiredStateFetcher {

	return &DesiredStateFetcher{
		config:           config,
		messageBus:       messageBus,
		httpClient:       httpClient,
		storeAdapter:     storeAdapter,
		freshnessManager: freshnessManager,
		timeProvider:     timeProvider,
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

		desiredState, err := NewDesiredStateServerResponse(body)
		if err != nil {
			resultChan <- DesiredStateFetcherResult{Message: "Failed to parse HTTP response body JSON", Error: err}
			return
		}
		if len(desiredState.Results) == 0 {
			fetcher.freshnessManager.Bump(fetcher.config.DesiredFreshnessKey, fetcher.config.DesiredFreshnessTTL, fetcher.timeProvider.Time())
			resultChan <- DesiredStateFetcherResult{Success: true, NumResults: numResults}
			return
		}

		err = fetcher.pushToStore(desiredState)
		if err != nil {
			resultChan <- DesiredStateFetcherResult{Message: "Failed to store desired state in store", Error: err}
			return
		}
		fetcher.fetchBatch(authorization, desiredState.BulkTokenRepresentation(), numResults+len(desiredState.Results), resultChan)
	})
}

func (fetcher *DesiredStateFetcher) bulkURL(batchSize int, bulkToken string) string {
	return fmt.Sprintf("%s/bulk/apps?batch_size=%d&bulk_token=%s", fetcher.config.CCBaseURL, batchSize, bulkToken)
}

func (fetcher *DesiredStateFetcher) pushToStore(desiredState DesiredStateServerResponse) error {
	nodes := make([]storeadapter.StoreNode, len(desiredState.Results))
	i := 0
	for _, desiredAppState := range desiredState.Results {
		nodes[i] = storeadapter.StoreNode{
			Key:   "/desired/" + desiredAppState.StoreKey(),
			Value: desiredAppState.ToJson(),
			TTL:   fetcher.config.DesiredStateTTL,
		}
		i++
	}

	err := fetcher.storeAdapter.Set(nodes)
	if err != nil {
		return err
	}

	return nil
}
