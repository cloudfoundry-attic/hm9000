package phd

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/actualstatelistener"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/bel_air"
	"github.com/cloudfoundry/hm9000/helpers/time_provider"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/test_helpers/app"
	"github.com/cloudfoundry/hm9000/test_helpers/fake_logger"

	"fmt"
	"time"
)

var _ = Describe("StorePerformance", func() {
	var (
		realStore store.Store
		listener  *actualstatelistener.ActualStateListener
	)

	BeforeEach(func() {
		realStore = store.NewETCDStore(config.ETCD_URL(4001))
		err := realStore.Connect()
		Ω(err).ShouldNot(HaveOccured())

		conf, err := config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())

		listener = actualstatelistener.New(
			conf,
			natsRunner.MessageBus,
			realStore,
			bel_air.NewFreshPrince(realStore),
			&time_provider.RealTimeProvider{},
			fake_logger.NewFakeLogger())
		listener.Start()
	})

	benchmarkActualStateWrite := func(numDeas int, numAppsPerDea int) {
		Context(fmt.Sprintf("\x1b[1m\x1b[32m%d DEAs each with %d apps (%d total)\x1b[0m", numDeas, numAppsPerDea, numDeas*numAppsPerDea), func() {
			var (
				instanceKeys   map[string]bool
				heartbeatJsons [][]byte
			)

			BeforeEach(func() {
				instanceKeys = make(map[string]bool, 0)
				heartbeatJsons = make([][]byte, 0)
				for i := 0; i < numDeas; i++ {
					dea := app.NewDea()
					heartbeat := dea.Heartbeat(numAppsPerDea, time.Now().Unix())
					heartbeatJson := heartbeat.ToJson()
					heartbeatJsons = append(heartbeatJsons, heartbeatJson)
					instanceKeys["/actual/"+dea.GetApp(numAppsPerDea-1).GetInstance(0).InstanceGuid] = false
				}
			})

			AfterEach(func() {
				values, err := realStore.List("/actual")
				Ω(err).ShouldNot(HaveOccured())
				Ω(values).Should(HaveLen(numDeas * numAppsPerDea))
			})

			Benchmark("writing to the store", func() {
				for _, heartbeatJson := range heartbeatJsons {
					natsRunner.MessageBus.Publish("dea.heartbeat", heartbeatJson)
				}

				Eventually(func() bool {
					for key, value := range instanceKeys {
						if value == false {
							_, err := realStore.Get(key)
							if err == nil {
								instanceKeys[key] = true
							} else {
								return false
							}
						}
					}
					return true
				}, 30.0, 0.05).Should(BeTrue(), "Didn't get all the keys")
			}, 5, 5.0)
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
				for i := 0; i < numApps; i++ {
					instance := app.NewApp().GetInstance(0)
					value := instance.Heartbeat(0).ToJson()
					realStore.Set("/actual/"+instance.InstanceGuid, value, 30)
				}
			})

			Benchmark("reading from the store", func() {
				values, err := realStore.List("/actual")
				Ω(err).ShouldNot(HaveOccured())
				Ω(values).Should(HaveLen(numApps))
			}, 3, 3.0)
		})
	}

	Describe("benchmarking reads", func() {
		for _, numApps := range []int{100, 1000, 3000, 10000, 30000} {
			benchmarkActualStateRead(numApps)
		}
	})
})
