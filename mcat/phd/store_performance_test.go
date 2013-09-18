package phd

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/test_helpers/app"
	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"

	"fmt"
	"time"
)

var _ = Describe("ETCD Store Performance", func() {
	for nodes := 1; nodes <= 7; nodes += 2 {
		nodes := nodes
		Context(fmt.Sprintf("With %d ETCD nodes", nodes), func() {
			var realStore store.Store

			BeforeEach(func() {
				etcdRunner = etcd_runner.NewETCDClusterRunner("etcd", 5001, nodes)
				etcdRunner.Start()

				realStore = store.NewETCDStore(etcdRunner.NodeURLS(), 100)
				err := realStore.Connect()
				Ω(err).ShouldNot(HaveOccured())
			})

			AfterEach(func() {
				realStore.Disconnect()
				etcdRunner.Stop()
				etcdRunner = nil
			})

			generateNodes := func(numApps int) []store.StoreNode {
				nodes := make([]store.StoreNode, numApps)

				heartbeat := app.NewDea().Heartbeat(numApps, time.Now().Unix())
				for i, instanceHeartbeat := range heartbeat.InstanceHeartbeats {
					nodes[i] = store.StoreNode{
						Key:   "/actual/" + instanceHeartbeat.InstanceGuid,
						Value: instanceHeartbeat.ToJson(),
						TTL:   0,
					}
				}

				return nodes
			}

			for _, numApps := range []int{100, 1000, 3000, 10000, 30000} {
				numApps := numApps
				Context("writing to the store", func() {
					var nodes []store.StoreNode

					BeforeEach(func() {
						nodes = generateNodes(numApps)
					})

					Benchmark(fmt.Sprintf("\x1b[1m\x1b[32m%d instance heartbeats\x1b[0m", numApps), func() {
						err := realStore.Set(nodes)
						Ω(err).ShouldNot(HaveOccured())
					}, 5, 60.0)

					AfterEach(func() {
						values, err := realStore.List("/actual")
						Ω(err).ShouldNot(HaveOccured())
						Ω(len(values)).Should(Equal(numApps))
					})
				})
			}

			for _, numApps := range []int{100, 1000, 3000, 10000, 30000} {
				numApps := numApps
				Context("reading from the store", func() {
					BeforeEach(func() {
						err := realStore.Set(generateNodes(numApps))
						Ω(err).ShouldNot(HaveOccured())
					})

					Benchmark(fmt.Sprintf("\x1b[1m\x1b[32m%d instance heartbeats\x1b[0m", numApps), func() {
						values, err := realStore.List("/actual")
						Ω(err).ShouldNot(HaveOccured())
						Ω(len(values)).Should(Equal(numApps))
					}, 5, 10.0)
				})
			}
		})
	}
})
