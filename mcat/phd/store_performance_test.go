package phd

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"

	"fmt"
	"time"
)

var storeTypes = []string{"ETCD", "Zookeeper"}
var nodeCounts = []int{1, 3, 5, 7}

var _ = Describe("Store Performance", func() {
	for _, storeType := range storeTypes {
		storeType := storeType
		for _, nodes := range nodeCounts {
			nodes := nodes
			Context(fmt.Sprintf("With %d %s nodes", nodes, storeType), func() {
				var storeAdapter storeadapter.StoreAdapter

				BeforeEach(func() {
					if storeType == "ETCD" {
						storeRunner = storerunner.NewETCDClusterRunner(5001, nodes)
						storeRunner.Start()

						storeAdapter = storeadapter.NewETCDStoreAdapter(storeRunner.NodeURLS(), 100)
						err := storeAdapter.Connect()
						Ω(err).ShouldNot(HaveOccured())
					} else if storeType == "Zookeeper" {
						storeRunner = storerunner.NewZookeeperClusterRunner(2181, nodes)
						storeRunner.Start()

						storeAdapter = storeadapter.NewZookeeperStoreAdapter(storeRunner.NodeURLS(), 100, &timeprovider.RealTimeProvider{}, time.Second)
						err := storeAdapter.Connect()
						Ω(err).ShouldNot(HaveOccured())
					}
				})

				AfterEach(func() {
					storeAdapter.Disconnect()
					storeRunner.Stop()
					storeRunner = nil
				})

				for _, numApps := range []int{100, 1000, 3000, 10000, 30000} {
					numApps := numApps

					Measure(fmt.Sprintf("Read/Write Performance With %d Apps", numApps), func(b Benchmarker) {
						data := make([]storeadapter.StoreNode, numApps)

						heartbeat := app.NewDea().Heartbeat(numApps, time.Now().Unix())
						for i, instanceHeartbeat := range heartbeat.InstanceHeartbeats {
							data[i] = storeadapter.StoreNode{
								Key:   "/actual/" + instanceHeartbeat.InstanceGuid,
								Value: instanceHeartbeat.ToJson(),
								TTL:   0,
							}
						}

						writeTime := b.Time("writing to the store", func() {
							err := storeAdapter.Set(data)
							Ω(err).ShouldNot(HaveOccured())
						}, StorePerformanceReport{StoreType: storeType, NumStoreNodes: nodes, NumApps: numApps, Subject: "write performance"})

						Ω(writeTime.Seconds()).Should(BeNumerically("<=", 30))

						readTime := b.Time("reading from the store", func() {
							values, err := storeAdapter.List("/actual")
							Ω(err).ShouldNot(HaveOccured())
							Ω(len(values)).Should(Equal(numApps), "Didn't find the correct number of entries in the store")
						}, StorePerformanceReport{StoreType: storeType, NumStoreNodes: nodes, NumApps: numApps, Subject: "read performance"})

						Ω(readTime.Seconds()).Should(BeNumerically("<=", 3))

						usage, err := storeRunner.DiskUsage()
						Ω(err).ShouldNot(HaveOccured())
						b.RecordValue("disk usage in MB", float64(usage)/1024.0/1024.0, StorePerformanceReport{StoreType: storeType, NumStoreNodes: nodes, NumApps: numApps, Subject: "disk usage"})
					}, 5)
				}
			})
		}
	}
})
