package phd_aws

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"

	"fmt"
)

var numberOfApps = []int{30, 100, 300, 1000, 3000, 10000}
var numberOfInstancesPerApp = 2

var _ = Describe("Benchmarking AWS MCAT ", func() {
	for _, numApps := range numberOfApps {
		numApps := numApps
		iteration := 1
		Context(fmt.Sprintf("With %d apps", numApps), func() {
			Measure("Read/Write/Delete Performance", func(b Benchmarker) {
				fmt.Printf("%d apps iteration %d\n", numApps, iteration)
				iteration += 1
				data := make([]storeadapter.StoreNode, numApps*numberOfInstancesPerApp)
				n := 0
				for i := 0; i < numApps; i++ {
					app := appfixture.NewAppFixture()
					for j := 0; j < numberOfInstancesPerApp; j++ {
						instance := app.InstanceAtIndex(j)
						data[n] = storeadapter.StoreNode{
							Key:   fmt.Sprintf("/apps/%s-%s/actual/%s", app.AppGuid, app.AppVersion, instance.InstanceGuid),
							Value: instance.Heartbeat().ToJSON(),
							TTL:   0,
						}
						n += 1
					}
				}

				b.Time("WRITE", func() {
					err := storeAdapter.Set(data)
					Ω(err).ShouldNot(HaveOccured())
				}, StorePerformanceReport{
					NumApps: numApps,
					Subject: "write",
				})

				b.Time("READ", func() {
					node, err := storeAdapter.ListRecursively("/apps")
					Ω(err).ShouldNot(HaveOccured())
					Ω(len(node.ChildNodes)).Should(Equal(numApps), "Didn't find the correct number of entries in the store")
				}, StorePerformanceReport{
					NumApps: numApps,
					Subject: "read",
				})

				b.Time("DELETE", func() {
					err := storeAdapter.Delete("/apps")
					Ω(err).ShouldNot(HaveOccured())
				}, StorePerformanceReport{
					NumApps: numApps,
					Subject: "delete",
				})
			}, 5)
		})
	}
})
