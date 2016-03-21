package desiredstateserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	. "github.com/cloudfoundry/hm9000/models"
)

type DesiredStateServerInterface interface {
	SpinUp()
	SetDesiredState([]DesiredAppState)
	URL() string
}

type DesiredStateServer struct {
	mu                      sync.Mutex
	Apps                    []DesiredAppState
	NumberOfCompleteFetches int
	port                    int
	url                     string
}

type DesiredStateServerResponse struct {
	Results   map[string]DesiredAppState `json:"results"`
	BulkToken struct {
		Id int `json:"id"`
	} `json:"bulk_token"`
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func NewDesiredStateServer(port int) *DesiredStateServer {
	return &DesiredStateServer{
		port: port,
		url:  fmt.Sprintf("http://127.0.0.1:%d", port),
	}
}

func (server *DesiredStateServer) URL() string {
	return server.url
}

func (server *DesiredStateServer) SpinUp() {
	http.HandleFunc("/bulk/apps", func(w http.ResponseWriter, r *http.Request) {
		server.handleApps(w, r)
	})

	http.HandleFunc("/bulk/counts", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"counts":{"user":17}}`)
	})

	http.ListenAndServe(fmt.Sprintf(":%d", server.port), nil)
}

func (server *DesiredStateServer) SetDesiredState(newState []DesiredAppState) {
	server.mu.Lock()
	server.Apps = newState
	server.mu.Unlock()
}

func (server *DesiredStateServer) Reset() {
	server.mu.Lock()
	server.Apps = make([]DesiredAppState, 0)
	server.NumberOfCompleteFetches = 0
	server.mu.Unlock()
}

func (server *DesiredStateServer) GetNumberOfCompleteFetches() int {
	return server.NumberOfCompleteFetches
}

func (server *DesiredStateServer) handleApps(w http.ResponseWriter, r *http.Request) {
	credentials := r.Header.Get("Authorization")
	if credentials != "Basic bWNhdDp0ZXN0aW5n" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	batchSize := server.extractBatchSize(r)
	bulkToken := server.extractBulkToken(r)
	server.mu.Lock()
	apps := server.Apps
	server.mu.Unlock()

	endIndex := bulkToken + batchSize
	endIndex = min(endIndex, len(apps))

	results := make(map[string]DesiredAppState, 0)

	for _, app := range apps[bulkToken:endIndex] {
		results[app.AppGuid] = app
	}

	if bulkToken == len(apps) {
		server.NumberOfCompleteFetches += 1
	}

	response := DesiredStateServerResponse{
		Results: results,
	}
	response.BulkToken.Id = endIndex
	json.NewEncoder(w).Encode(response)
}

func (server *DesiredStateServer) extractBatchSize(r *http.Request) int {
	batchSize, _ := strconv.Atoi(r.URL.Query()["batch_size"][0])
	return batchSize
}

func (server *DesiredStateServer) extractBulkToken(r *http.Request) int {
	var bulkToken map[string]interface{}
	json.Unmarshal([]byte(r.URL.Query()["bulk_token"][0]), &bulkToken)

	bulkTokenIndex := 0
	if bulkToken["id"] != nil {
		bulkTokenIndex = int(bulkToken["id"].(float64))
	}

	return bulkTokenIndex
}
