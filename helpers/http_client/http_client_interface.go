package http_client

import "net/http"

type HttpClient interface {
	Do(req *http.Request, callback func(*http.Response, error))
}
