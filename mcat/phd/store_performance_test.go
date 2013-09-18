package phd

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	"github.com/cloudfoundry/hm9000/testhelpers/etcdrunner"

	"fmt"
	"strings"
	"time"
)

type Report struct {
	NumStoreNodes int
	NumApps       int
	Subject       string
}

func (r Report) String() string {
	return fmt.Sprintf("%s: %d ETCD node(s), %d apps", strings.Title(r.Subject), r.NumStoreNodes, r.NumApps)
}

var _ = Describe("ETCD Store Performance", func() {
	for nodes := 1; nodes <= 7; nodes += 2 {
		nodes := nodes
		Context(fmt.Sprintf("With %d ETCD nodes", nodes), func() {
			var realStore store.Store

			BeforeEach(func() {
				etcdRunner = etcdrunner.NewETCDClusterRunner("etcd", 5001, nodes)
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

			for _, numApps := range []int{100, 1000, 3000, 10000, 30000} {
				numApps := numApps

				Measure(fmt.Sprintf("Read/Write Performance With %d Apps", numApps), func(b Benchmarker) {
					data := make([]store.StoreNode, numApps)

					heartbeat := app.NewDea().Heartbeat(numApps, time.Now().Unix())
					for i, instanceHeartbeat := range heartbeat.InstanceHeartbeats {
						data[i] = store.StoreNode{
							Key:   "/actual/" + instanceHeartbeat.InstanceGuid,
							Value: instanceHeartbeat.ToJson(),
							TTL:   0,
						}
					}

					writeTime := b.Time("writing to the store", func() {
						err := realStore.Set(data)
						Ω(err).ShouldNot(HaveOccured())
					}, Report{NumStoreNodes: nodes, NumApps: numApps, Subject: "write performance"})

					Ω(writeTime.Seconds()).Should(BeNumerically("<=", 30))

					readTime := b.Time("reading from the store", func() {
						values, err := realStore.List("/actual")
						Ω(err).ShouldNot(HaveOccured())
						Ω(len(values)).Should(Equal(numApps), "Didn't find the correct number of entries in the store")
					}, Report{NumStoreNodes: nodes, NumApps: numApps, Subject: "read performance"})

					Ω(readTime.Seconds()).Should(BeNumerically("<=", 3))

					usage, err := etcdRunner.DiskUsage()
					Ω(err).ShouldNot(HaveOccured())
					b.RecordValue("disk usage in MB", float64(usage)/1024.0/1024.0, Report{NumStoreNodes: nodes, NumApps: numApps, Subject: "disk usage"})
				}, 5)
			}
		})
	}
})
