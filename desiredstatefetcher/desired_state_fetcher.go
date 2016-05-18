package desiredstatefetcher

import (
	"errors"
	"fmt"

	"io/ioutil"
	"net/http"
	"strings"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/httpclient"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

type DesiredStateFetcherResult struct {
	Success    bool
	Message    string
	Error      error
	NumResults int
}

const initialBulkToken = "{}"

type DesiredStateFetcher struct {
	config     *config.Config
	httpClient httpclient.HttpClient
	clock      clock.Clock
	logger     lager.Logger
}

func New(config *config.Config,
	httpClient httpclient.HttpClient,
	clock clock.Clock,
	logger lager.Logger,
) *DesiredStateFetcher {

	return &DesiredStateFetcher{
		config:     config,
		httpClient: httpClient,
		clock:      clock,
		logger:     logger,
	}
}

func (fetcher *DesiredStateFetcher) Fetch(resultChan chan DesiredStateFetcherResult, appQueue *models.AppQueue) {
	authInfo := models.BasicAuthInfo{
		User:     fetcher.config.CCAuthUser,
		Password: fetcher.config.CCAuthPassword,
	}

	fetcher.fetchBatch(authInfo.Encode(), initialBulkToken, 0, resultChan, appQueue)
}

func (fetcher *DesiredStateFetcher) fetchBatch(authorization string, token string, numResults int, resultChan chan DesiredStateFetcherResult, appQueue *models.AppQueue) {
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

		body, err := ioutil.ReadAll(resp.Body)
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
			resultChan <- DesiredStateFetcherResult{Success: true, NumResults: numResults}
			return
		}
		err = fetcher.sendBatch(response, appQueue)
		if err != nil {
			resultChan <- DesiredStateFetcherResult{Message: "Stopping fetcher", Error: err}
			return
		}
		fetcher.fetchBatch(authorization, response.BulkTokenRepresentation(), numResults+len(response.Results), resultChan, appQueue)
	})
}

func (fetcher *DesiredStateFetcher) bulkURL(batchSize int, bulkToken string) string {
	return fmt.Sprintf("%s/bulk/apps?batch_size=%d&bulk_token=%s", fetcher.config.CCBaseURL, batchSize, bulkToken)
}

func (fetcher *DesiredStateFetcher) guids(desiredStates []models.DesiredAppState) string {
	result := make([]string, len(desiredStates))

	for i, desired := range desiredStates {
		result[i] = desired.AppGuid
	}

	return strings.Join(result, ",")
}

func (fetcher *DesiredStateFetcher) sendBatch(response DesiredStateServerResponse, appQueue *models.AppQueue) error {
	cache := map[string]models.DesiredAppState{}
	for _, desiredState := range response.Results {
		if desiredState.State == models.AppStateStarted && (desiredState.PackageState == models.AppPackageStateStaged || desiredState.PackageState == models.AppPackageStatePending) {
			cache[desiredState.StoreKey()] = desiredState
		}
	}

	select {
	case <-appQueue.DoneAnalyzing:
		return errors.New("Analyzer is not analyzing")
	case appQueue.DesiredApps <- cache:
	}
	return nil
}
