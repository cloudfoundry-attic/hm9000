package http_client

import "net/http"

type HttpClientFactory interface {
	NewClient() HttpClient
}

type HttpClient interface {
	Do(req *http.Request) chan HttpResult
}

type HttpResult struct {
	Response *http.Response
	Err      error
}
