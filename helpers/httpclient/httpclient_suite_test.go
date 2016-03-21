package httpclient_test

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var badUrl string
var url string

var tcpListen net.Listener
var httpListen net.Listener

func TestHttpclient(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Httpclient Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	return nil
}, func([]byte) {
	var err error
	tcpPort := 8880 + GinkgoParallelNode()
	tcpListen, err = net.Listen("tcp", fmt.Sprintf(":%d", tcpPort))
	Expect(err).NotTo(HaveOccurred())
	badUrl = fmt.Sprintf("http://127.0.0.1:%d", tcpPort)

	http.HandleFunc("/sleep", func(w http.ResponseWriter, r *http.Request) {
		sleepTimeInSeconds, _ := strconv.ParseFloat(r.URL.Query().Get("time"), 64)
		time.Sleep(time.Duration(sleepTimeInSeconds * float64(time.Second)))
		fmt.Fprintf(w, "I'm awake!")
	})

	httpPort := 8980 + GinkgoParallelNode()
	url = fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	httpListen, err = net.Listen("tcp", fmt.Sprintf(":%d", httpPort))
	Expect(err).NotTo(HaveOccurred())
	go func() {
		defer GinkgoRecover()
		server := &http.Server{}
		_ = server.Serve(httpListen)
	}()

	Eventually(func() error {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", httpPort), 100*time.Millisecond)
		if err == nil {
			c.Close()
		}
		return err
	}).Should(BeNil())
})

var _ = SynchronizedAfterSuite(func() {
	if tcpListen != nil {
		tcpListen.Close()
	}

	if httpListen != nil {
		httpListen.Close()
	}
}, func() {

})
