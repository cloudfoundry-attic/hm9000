package http_client

import "net/http"

func NewHttpClient() HttpClient {
	return &RealHttpClient{
		client: &http.Client{},
	}
}

type RealHttpClient struct {
	client *http.Client
}

func (client *RealHttpClient) Do(req *http.Request, callback func(*http.Response, error)) {
	response, err := client.client.Do(req)
	callback(response, err)
}
