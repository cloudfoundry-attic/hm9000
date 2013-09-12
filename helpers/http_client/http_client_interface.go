package http_client

import "net/http"

type HttpClientFactory interface {
	NewClient() HttpClient
}

type HttpClient interface {
	Do(req *http.Request) chan HttpResponseErr
}

type HttpResponseErr struct {
	Response *http.Response
	Err      error
}
