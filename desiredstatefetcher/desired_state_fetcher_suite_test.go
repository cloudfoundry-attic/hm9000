package desiredstatefetcher_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cloudfoundry/hm9000/testhelpers/desiredstateserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var stateServer *desiredstateserver.DesiredStateServer

func TestDesiredStateFetcher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Desired State Fetcher Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	return nil
}, func([]byte) {
	port := 6001 + GinkgoParallelNode()
	stateServer = desiredstateserver.NewDesiredStateServer(port)
	go stateServer.SpinUp()

	url := fmt.Sprintf("%s/bulk/counts", stateServer.URL())
	Eventually(func() int {
		before := time.Now()
		resp, err := http.Get(url)
		println("GET", time.Now().Sub(before).String())
		if err != nil {
			return 0
		}
		return resp.StatusCode
	}).Should(Equal(http.StatusOK))
})
