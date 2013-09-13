package desiredstatepoller

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

const initialBulkToken = "{}"

type desiredStatePoller struct {
	messageBus    cfmessagebus.MessageBus
	httpClient    http_client.HttpClient
	store         store.Store
	freshPrince   bel_air.FreshPrince
	timeProvider  time_provider.TimeProvider
	authorization string
	ccBaseURL     string
	batchSize     int
}

func NewDesiredStatePoller(messageBus cfmessagebus.MessageBus,
	store store.Store,
	httpClient http_client.HttpClient,
	freshPrince bel_air.FreshPrince,
	timeProvider time_provider.TimeProvider,
	ccBaseURL string,
	batchSize int) *desiredStatePoller {

	return &desiredStatePoller{
		messageBus:   messageBus,
		httpClient:   httpClient,
		store:        store,
		freshPrince:  freshPrince,
		timeProvider: timeProvider,
		ccBaseURL:    ccBaseURL,
		batchSize:    batchSize,
	}
}

func (poller *desiredStatePoller) Poll() {
	if poller.authenticated() {
		poller.fetch()
	} else {
		poller.messageBus.Request("cloudcontroller.bulk.credentials.default", []byte{}, func(response []byte) {
			authInfo, err := models.NewBasicAuthInfoFromJSON(response)
			if err != nil {
				//TODO: Log
				return
			}

			poller.authorization = authInfo.Encode()
			poller.fetch()
		})
	}
}

func (poller *desiredStatePoller) authenticated() bool {
	return poller.authorization != ""
}

func (poller *desiredStatePoller) bulkURL(batchSize int, bulkToken string) string {
	return fmt.Sprintf("%s/bulk/apps?batch_size=%d&bulk_token=%s", poller.ccBaseURL, batchSize, bulkToken)
}

func (poller *desiredStatePoller) fetch() {
	poller.fetchBatch(initialBulkToken)
}

func (poller *desiredStatePoller) fetchBatch(token string) {
	req, err := http.NewRequest("GET", poller.bulkURL(poller.batchSize, token), nil)

	if err != nil {
		//TODO: Log
		return
	}

	req.Header.Add("Authorization", poller.authorization)

	poller.httpClient.Do(req, func(resp *http.Response, err error) {
		if err != nil {
			//TODO: Log
			return
		}

		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			poller.authorization = ""
			//TODO: Log
			return
		}

		body := make([]byte, resp.ContentLength)
		_, err = resp.Body.Read(body)

		if err != nil {
			//TODO: Log
			return
		}

		desiredState, err := NewDesiredStateServerResponse(body)
		if err != nil {
			return
		}
		if len(desiredState.Results) == 0 {
			poller.freshPrince.Bump(config.DESIRED_FRESHNESS_KEY, config.DESIRED_FRESHNESS_TTL, poller.timeProvider.Time())
			return
		}

		poller.pushToStore(desiredState)
		poller.fetchBatch(desiredState.BulkTokenRepresentation())
	})
}

func (poller *desiredStatePoller) pushToStore(desiredState desiredStateServerResponse) {
	for _, desiredAppState := range desiredState.Results {
		key := "/desired/" + desiredAppState.AppGuid + "-" + desiredAppState.AppVersion
		poller.store.Set(key, desiredAppState.ToJson(), config.DESIRED_STATE_TTL)
	}
}
