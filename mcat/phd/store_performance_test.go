package phd

import (
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/test_helpers/app"

	"fmt"
	"time"
)

var _ = Describe("StorePerformance", func() {
	var (
		realStore store.Store
	)

	BeforeEach(func() {
		realStore = store.NewETCDStore(etcdRunner.NodeURLS(), 100)
		err := realStore.Connect()
		Ω(err).ShouldNot(HaveOccured())
	})

	AfterEach(func() {
		realStore.Disconnect()
	})

	benchmarkActualStateWrite := func(numDeas int, numAppsPerDea int) {
		Context(fmt.Sprintf("\x1b[1m\x1b[32m%d DEAs each with %d apps (%d total)\x1b[0m", numDeas, numAppsPerDea, numDeas*numAppsPerDea), func() {
			var (
				heartbeats []models.Heartbeat
			)

			BeforeEach(func() {
				heartbeats = make([]models.Heartbeat, numDeas)
				for i := 0; i < numDeas; i++ {
					heartbeats[i] = app.NewDea().Heartbeat(numAppsPerDea, time.Now().Unix())
				}
			})

			AfterEach(func() {
				values, err := realStore.List("/actual")
				Ω(err).ShouldNot(HaveOccured())
				Ω(len(values)).Should(Equal(numDeas * numAppsPerDea))
			})

			Benchmark("writing to the store (and reading the entire sample)", func() {
				completions := make(chan bool, len(heartbeats))
				for _, heartbeat := range heartbeats {
					nodes := make([]store.StoreNode, len(heartbeat.InstanceHeartbeats))
					for i, instance := range heartbeat.InstanceHeartbeats {
						nodes[i] = store.StoreNode{
							Key:   "/actual/" + instance.InstanceGuid,
							Value: instance.ToJson(),
							TTL:   0,
						}
					}
					go func() {
						err := realStore.Set(nodes)
						Ω(err).ShouldNot(HaveOccured())
						completions <- true
					}()
				}

				Eventually(func() chan bool {
					return completions
				}, 60.0, 0.01).Should(HaveLen(len(heartbeats)), "Never finished writing to store")
			}, 5, 60.0)
		})
	}

	Describe("benchmarking writes", func() {
		Context("scaling the number of DEAs", func() {
			for _, numDeas := range []int{1, 10, 30, 100, 300} {
				benchmarkActualStateWrite(numDeas, 100)
			}
		})

		Context("scaling the number of Apps per Dea", func() {
			// NATS doesn't let us go past 3000
			for _, numAppsPerDea := range []int{1000, 2000, 3000} {
				benchmarkActualStateWrite(1, numAppsPerDea)
			}
		})
	})

	benchmarkActualStateRead := func(numApps int) {
		Context(fmt.Sprintf("\x1b[1m\x1b[32m%d apps\x1b[0m", numApps), func() {
			BeforeEach(func() {
				nodes := make([]store.StoreNode, numApps)
				for i := 0; i < numApps; i++ {
					instance := app.NewApp().GetInstance(0)
					nodes[i] = store.StoreNode{
						Key:   "/actual/" + instance.InstanceGuid,
						Value: instance.Heartbeat(0).ToJson(),
						TTL:   0,
					}
				}
				err := realStore.Set(nodes)
				Ω(err).ShouldNot(HaveOccured())
			})

			Benchmark("reading from the store", func() {
				values, err := realStore.List("/actual")
				Ω(err).ShouldNot(HaveOccured())
				Ω(values).Should(HaveLen(numApps))
			}, 3, 10.0)
		})
	}

	Describe("benchmarking reads", func() {
		for _, numApps := range []int{100, 1000, 3000, 10000, 30000} {
			benchmarkActualStateRead(numApps)
		}
	})
})
