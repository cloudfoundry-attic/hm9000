package phd_aws

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/models"
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
				data := make([]models.InstanceHeartbeat, numApps*numberOfInstancesPerApp)
				n := 0
				for i := 0; i < numApps; i++ {
					app := appfixture.NewAppFixture()
					for j := 0; j < numberOfInstancesPerApp; j++ {
						data[n] = app.InstanceAtIndex(j).Heartbeat()
						n += 1
					}
				}

				b.Time("WRITE", func() {
					err := store.SaveActualState(data...)
					Ω(err).ShouldNot(HaveOccured())
				}, StorePerformanceReport{
					NumApps: numApps,
					Subject: "write",
				})

				b.Time("READ", func() {
					nodes, err := store.GetActualState()
					Ω(err).ShouldNot(HaveOccured())
					Ω(len(nodes)).Should(Equal(numApps*numberOfInstancesPerApp), "Didn't find the correct number of entries in the store")
				}, StorePerformanceReport{
					NumApps: numApps,
					Subject: "read",
				})

				b.Time("DELETE", func() {
					err := store.TruncateActualState()
					Ω(err).ShouldNot(HaveOccured())
				}, StorePerformanceReport{
					NumApps: numApps,
					Subject: "delete",
				})
			}, 5)
		})
	}
})
