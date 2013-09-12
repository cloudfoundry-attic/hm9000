package http_client

import "net/http"

type RealHttpClientFactory struct{}

func (factory *RealHttpClientFactory) NewClient() HttpClient {
	return &RealHttpClient{
		client: &http.Client{},
	}
}

type RealHttpClient struct {
	client *http.Client
}

func (client *RealHttpClient) Do(req *http.Request) chan HttpResult {
	c := make(chan HttpResult, 1)
	go func() {
		resp, err := client.client.Do(req)
		c <- HttpResult{
			Response: resp,
			Err:      err,
		}
	}()
	return c
}
