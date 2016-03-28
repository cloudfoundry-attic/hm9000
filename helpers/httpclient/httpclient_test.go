package httpclient_test

import (
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/cloudfoundry/hm9000/helpers/httpclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Httpclient", func() {
	var client HttpClient

	BeforeEach(func() {
		client = NewHttpClient(true, 10*time.Millisecond)
	})

	Context("when the request times out (trying to connect)", func() {
		It("should return an appropriate timeout error", func() {
			request, _ := http.NewRequest("GET", badUrl, nil)
			client.Do(request, func(response *http.Response, err error) {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("when the request times out (after conecting)", func() {
		It("should return an appropriate timeout error", func() {
			request, _ := http.NewRequest("GET", url+"/sleep?time=1", nil)
			client.Do(request, func(response *http.Response, err error) {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("when the request does not time out", func() {
		It("should return the correct response", func() {
			request, _ := http.NewRequest("GET", url+"/sleep?time=0", nil)
			client.Do(request, func(response *http.Response, err error) {
				Expect(err).NotTo(HaveOccurred())
				defer response.Body.Close()
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(body)).To(Equal("I'm awake!"))
			})
		})
	})
})
