package actualstatelistener

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"
	"time"

	. "github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/test_helpers/app"
	"github.com/cloudfoundry/hm9000/test_helpers/fake_bel_air"
	"github.com/cloudfoundry/hm9000/test_helpers/fake_logger"
	"github.com/cloudfoundry/hm9000/test_helpers/fake_time_provider"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/store"
)

var _ = Describe("Actual state listener", func() {
	var (
		app          App
		anotherApp   App
		etcdStore    store.Store
		listener     *ActualStateListener
		timeProvider *fake_time_provider.FakeTimeProvider
		freshPrince  *fake_bel_air.FakeFreshPrince
	)

	BeforeEach(func() {
		timeProvider = &fake_time_provider.FakeTimeProvider{
			TimeToProvide: time.Now(),
		}

		app = NewApp()
		anotherApp = NewApp()

		etcdStore = store.NewETCDStore(config.ETCD_URL(4001))
		err := etcdStore.Connect()
		Ω(err).ShouldNot(HaveOccured())

		freshPrince = &fake_bel_air.FakeFreshPrince{}

		listener = NewActualStateListener(natsRunner.MessageBus, etcdStore, freshPrince, timeProvider, fake_logger.NewFakeLogger())
		listener.Start()
	})

	verifyHeartbeatInStore := func(hb InstanceHeartbeat) {
		storeKey := "/actual/" + hb.InstanceGuid
		var value store.StoreNode
		var err error

		Eventually(func() interface{} {
			value, err = etcdStore.Get(storeKey)
			return err
		}, 0.1, 0.01).ShouldNot(HaveOccured())

		Ω(value.TTL).Should(BeNumerically("==", config.HEARTBEAT_TTL-1), "TTL starts decrementing immediately")
		Ω(value.Key).Should(Equal(storeKey))

		var instanceHeartbeat InstanceHeartbeat
		json.Unmarshal([]byte(value.Value), &instanceHeartbeat)

		Ω(instanceHeartbeat).Should(Equal(hb))
	}

	Context("When it receives a simple heartbeat on NATS", func() {
		BeforeEach(func() {
			messagePublisher.PublishHeartbeat(app.Heartbeat(1, 17))
		})

		It("Stores it in the store", func() {
			verifyHeartbeatInStore(app.GetInstance(0).Heartbeat(17))
		})
	})

	Context("When it receives a complex heartbeat with multiple apps and instances", func() {
		BeforeEach(func() {
			heartbeat := Heartbeat{
				DeaGuid: Guid(),
				InstanceHeartbeats: []InstanceHeartbeat{
					app.GetInstance(0).Heartbeat(17),
					app.GetInstance(1).Heartbeat(22),
					anotherApp.GetInstance(0).Heartbeat(11),
				},
			}

			messagePublisher.PublishHeartbeat(heartbeat)
		})

		It("Stores it in the store", func() {
			verifyHeartbeatInStore(app.GetInstance(0).Heartbeat(17))
			verifyHeartbeatInStore(app.GetInstance(1).Heartbeat(22))
			verifyHeartbeatInStore(anotherApp.GetInstance(0).Heartbeat(11))
		})
	})

	Describe("freshness", func() {
		Context("when a heartbeat arrives", func() {
			BeforeEach(func() {
				messagePublisher.PublishHeartbeat(app.Heartbeat(1, 17))
			})

			It("should create /actual-fresh with the current timestamp and a TTL", func() {
				Eventually(func() interface{} {
					return freshPrince.Key
				}, 0.1, 0.01).Should(Equal(config.ACTUAL_FRESHNESS_KEY))

				Ω(freshPrince.Timestamp).Should(Equal(timeProvider.Time()))
				Ω(freshPrince.TTL).Should(BeNumerically("==", config.ACTUAL_FRESHNESS_TTL))
			})
		})
	})
})
