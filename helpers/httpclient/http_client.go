package httpclient

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

func NewHttpClient(skipSSLVerification bool, timeout time.Duration) HttpClient {
	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: timeout,
		}).Dial,

		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipSSLVerification,
		},
	}

	return &RealHttpClient{
		client: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
	}
}

type RealHttpClient struct {
	client *http.Client
}

func (client *RealHttpClient) Do(req *http.Request, callback func(*http.Response, error)) {
	response, err := client.client.Do(req)
	callback(response, err)
}
