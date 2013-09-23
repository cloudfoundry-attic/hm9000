package phd

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"

	"fmt"
	"time"
)

var maxNumRecords = 1000
var maxMemoryInMB = 100
var storeTypes = []string{"ETCD", "Zookeeper"}
var nodeCounts = []int{1, 3, 5, 7}
var concurrencies = []int{1, 10, 100, 300, 1000}
var recordSizes = []int{1, 3, 10, 30}

var _ = Describe("Detailed Store Performance", func() {
	for _, storeType := range storeTypes {
		storeType := storeType
		for _, nodes := range nodeCounts {
			nodes := nodes
			for _, concurrency := range concurrencies {
				concurrency := concurrency
				Context(fmt.Sprintf("With %d %s nodes (%d concurrent requests at a time)", nodes, storeType, concurrency), func() {
					var storeAdapter storeadapter.StoreAdapter

					BeforeEach(func() {
						if storeType == "ETCD" {
							storeRunner = storerunner.NewETCDClusterRunner(5001, nodes)
							storeRunner.Start()

							storeAdapter = storeadapter.NewETCDStoreAdapter(storeRunner.NodeURLS(), concurrency)
							err := storeAdapter.Connect()
							Ω(err).ShouldNot(HaveOccured())
						} else if storeType == "Zookeeper" {
							storeRunner = storerunner.NewZookeeperClusterRunner(2181, nodes)
							storeRunner.Start()

							storeAdapter = storeadapter.NewZookeeperStoreAdapter(storeRunner.NodeURLS(), concurrency, &timeprovider.RealTimeProvider{}, time.Second)
							err := storeAdapter.Connect()
							Ω(err).ShouldNot(HaveOccured())
						}
					})

					AfterEach(func() {
						storeAdapter.Disconnect()
						storeRunner.Stop()
						storeRunner = nil
					})

					for _, recordSize := range recordSizes {
						recordSize := recordSize
						numRecords := maxMemoryInMB * 1024 / recordSize
						if numRecords > maxNumRecords {
							numRecords = maxNumRecords
						}

						Measure(fmt.Sprintf("Read/Write Performance With record size: %d (will generate %d records)", recordSize, numRecords), func(b Benchmarker) {
							data := make([]storeadapter.StoreNode, numRecords)
							for i := 0; i < numRecords; i++ {
								data[i] = storeadapter.StoreNode{
									Key:   fmt.Sprintf("/record/%d", i),
									Value: make([]byte, recordSize*1024),
									TTL:   0,
								}
							}

							b.Time("writing to the store", func() {
								err := storeAdapter.Set(data)
								Ω(err).ShouldNot(HaveOccured())
							}, StorePerformanceReport{
								Subject:       "write",
								StoreType:     storeType,
								NumStoreNodes: nodes,
								RecordSize:    recordSize,
								NumRecords:    numRecords,
								Concurrency:   concurrency,
							})

							b.Time("reading from the store", func() {
								values, err := storeAdapter.List("/record")
								Ω(err).ShouldNot(HaveOccured())
								Ω(len(values)).Should(Equal(numRecords), "Didn't find the correct number of entries in the store")
							}, StorePerformanceReport{
								Subject:       "read",
								StoreType:     storeType,
								NumStoreNodes: nodes,
								RecordSize:    recordSize,
								NumRecords:    numRecords,
								Concurrency:   concurrency,
							})
						}, 5)
					}
				})
			}
		}
	}
})
