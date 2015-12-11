package desiredstatefetcher_test

import (
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

var _ = BeforeSuite(func() {
	stateServer = desiredstateserver.NewDesiredStateServer()
	go stateServer.SpinUp(6001)

	Eventually(func() int {
		before := time.Now()
		resp, err := http.Get("http://127.0.0.1:6001/bulk/counts")
		println("GET", time.Now().Sub(before).String())
		if err != nil {
			return 0
		}
		return resp.StatusCode
	}).Should(Equal(http.StatusOK))
})
