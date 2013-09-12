package fake_http_client

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

type FakeHttpClient struct {
	Requests []*Request
}

type Request struct {
	*http.Request
	Callback func(*http.Response, error)
}

func NewFakeHttpClient() *FakeHttpClient {
	client := &FakeHttpClient{}
	client.Reset()
	return client
}

func (client *FakeHttpClient) Reset() {
	client.Requests = make([]*Request, 0)
}

func (client *FakeHttpClient) LastRequest() *Request {
	return client.Requests[len(client.Requests)-1]
}

func (client *FakeHttpClient) Do(req *http.Request, callback func(*http.Response, error)) {
	client.Requests = append(client.Requests, &Request{
		Request:  req,
		Callback: callback,
	})
}

func (request *Request) RespondWithStatus(statusCode int) {
	request.Respond(statusCode, "", nil)
}

func (request *Request) Succeed(body string) {
	request.Respond(http.StatusOK, body, nil)
}

func (request *Request) Respond(statusCode int, body string, err error) {
	reader := strings.NewReader(body)
	response := &http.Response{
		Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
		StatusCode: statusCode,

		ContentLength: int64(reader.Len()),
		Body:          ioutil.NopCloser(reader),
	}

	request.Callback(response, err)
}
