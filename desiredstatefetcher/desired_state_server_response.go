package desiredstatefetcher

import (
	"encoding/json"
	"github.com/cloudfoundry/hm9000/models"
)

type desiredStateServerResponse struct {
	Results   map[string]models.DesiredAppState `json:"results"`
	BulkToken bulkToken                         `json:"bulk_token"`
}

type bulkToken struct {
	Id int `json:"id"`
}

func NewDesiredStateServerResponse(jsonMessage []byte) (desiredStateServerResponse, error) {
	response := desiredStateServerResponse{}
	err := json.Unmarshal(jsonMessage, &response)
	return response, err
}

func (response desiredStateServerResponse) BulkTokenRepresentation() string {
	bulkTokenRepresentation, _ := json.Marshal(response.BulkToken)
	return string(bulkTokenRepresentation)
}

func (response desiredStateServerResponse) ToJson() []byte {
	encoded, _ := json.Marshal(response)
	return encoded
}
