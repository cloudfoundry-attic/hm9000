package desiredstatefetcher

import (
	"errors"
	"fmt"
	"os"

	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/httpclient"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
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
	config            *config.Config
	httpClient        httpclient.HttpClient
	store             store.Store
	metricsAccountant metricsaccountant.MetricsAccountant
	clock             clock.Clock
	cache             map[string]models.DesiredAppState
	logger            lager.Logger
}

func New(config *config.Config,
	store store.Store,
	metricsAccountant metricsaccountant.MetricsAccountant,
	httpClient httpclient.HttpClient,
	clock clock.Clock,
	logger lager.Logger) *DesiredStateFetcher {

	return &DesiredStateFetcher{
		config:            config,
		httpClient:        httpClient,
		store:             store,
		metricsAccountant: metricsAccountant,
		clock:             clock,
		cache:             map[string]models.DesiredAppState{},
		logger:            logger,
	}
}

func (fetcher *DesiredStateFetcher) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)
	for {
		afterChan := time.After(fetcher.config.FetcherPollingInterval())
		timeoutChan := time.After(fetcher.config.FetcherTimeout())
		errorChan := make(chan error, 1)

		t := time.Now()

		go func() {
			resultChan := make(chan DesiredStateFetcherResult, 1)
			fetcher.Fetch(resultChan)
			result := <-resultChan
			if result.Success {
				errorChan <- nil
			} else {
				fetcher.logger.Error(result.Message, result.Error)
				errorChan <- result.Error
			}
		}()

		select {
		case err := <-errorChan:
			fetcher.logger.Info("ifrit time", lager.Data{
				"Component": "fetcher",
				"Duration":  fmt.Sprintf("%.4f", time.Since(t).Seconds()),
			})
			if err != nil {
				fetcher.logger.Error("Fetcher returned an error. Continuing...", err)
			}
		case <-timeoutChan:
			return errors.New("Fetcher timed out. Aborting!")
		case <-signals:
			return nil
		}

		<-afterChan
	}
}

func (fetcher *DesiredStateFetcher) Fetch(resultChan chan DesiredStateFetcherResult) {
	fetcher.cache = map[string]models.DesiredAppState{}

	authInfo := models.BasicAuthInfo{
		User:     fetcher.config.CCAuthUser,
		Password: fetcher.config.CCAuthPassword,
	}

	fetcher.fetchBatch(authInfo.Encode(), initialBulkToken, 0, resultChan)
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
			tSync := fetcher.clock.Now()
			err = fetcher.syncStore()
			fetcher.metricsAccountant.TrackDesiredStateSyncTime(time.Since(tSync))
			if err != nil {
				resultChan <- DesiredStateFetcherResult{Message: "Failed to sync desired state to the store", Error: err}
				return
			}

			fetcher.store.BumpDesiredFreshness(fetcher.clock.Now())
			resultChan <- DesiredStateFetcherResult{Success: true, NumResults: numResults}
			return
		}

		fetcher.cacheResponse(response)
		fetcher.fetchBatch(authorization, response.BulkTokenRepresentation(), numResults+len(response.Results), resultChan)
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

func (fetcher *DesiredStateFetcher) syncStore() error {
	desiredStates := make([]models.DesiredAppState, len(fetcher.cache))
	i := 0
	for _, desiredState := range fetcher.cache {
		desiredStates[i] = desiredState
		i++
	}
	err := fetcher.store.SyncDesiredState(desiredStates...)
	if err != nil {
		fetcher.logger.Error("Failed to Sync Desired State", err, lager.Data{
			"Number of Entries": strconv.Itoa(len(desiredStates)),
			"Desireds":          fetcher.guids(desiredStates),
		})
		return err
	}

	return nil
}

func (fetcher *DesiredStateFetcher) cacheResponse(response DesiredStateServerResponse) {
	for _, desiredState := range response.Results {
		if desiredState.State == models.AppStateStarted && (desiredState.PackageState == models.AppPackageStateStaged || desiredState.PackageState == models.AppPackageStatePending) {
			fetcher.cache[desiredState.StoreKey()] = desiredState
		}
	}
}
