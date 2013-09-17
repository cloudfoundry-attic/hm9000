package actualstatelistener

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"

	. "github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/test_helpers/app"

	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/test_helpers/fake_bel_air"
	"github.com/cloudfoundry/hm9000/test_helpers/fake_logger"
	"github.com/cloudfoundry/hm9000/test_helpers/fake_time_provider"
)

var _ = Describe("Actual state listener", func() {
	var (
		app          App
		anotherApp   App
		etcdStore    store.Store
		listener     *ActualStateListener
		timeProvider *fake_time_provider.FakeTimeProvider
		freshPrince  *fake_bel_air.FakeFreshPrince
		messageBus   *fake_cfmessagebus.FakeMessageBus
		conf         config.Config
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

		messageBus = fake_cfmessagebus.NewFakeMessageBus()

		freshPrince = &fake_bel_air.FakeFreshPrince{}

		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())

		listener = New(conf, messageBus, etcdStore, freshPrince, timeProvider, fake_logger.NewFakeLogger())
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

		Ω(value.TTL).Should(BeNumerically("==", conf.HeartbeatTTL-1), "TTL starts decrementing immediately")
		Ω(value.Key).Should(Equal(storeKey))

		instanceHeartbeat, err := NewInstanceHeartbeatFromJSON(value.Value)
		Ω(err).ShouldNot(HaveOccured())
		Ω(instanceHeartbeat).Should(Equal(hb))
	}

	It("should subscribe to the dea.heartbeat subject", func() {
		Ω(messageBus.Subscriptions).Should(HaveKey("dea.heartbeat"))
		Ω(messageBus.Subscriptions["dea.heartbeat"]).Should(HaveLen(1))
	})

	Context("When it receives a simple heartbeat over the message bus", func() {
		BeforeEach(func() {
			messageBus.Subscriptions["dea.heartbeat"][0].Callback(app.Heartbeat(1, 17).ToJson())
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

			messageBus.Subscriptions["dea.heartbeat"][0].Callback(heartbeat.ToJson())
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
				messageBus.Subscriptions["dea.heartbeat"][0].Callback(app.Heartbeat(1, 17).ToJson())
			})

			It("should instruct the prince to bump the freshness", func() {
				Eventually(func() interface{} {
					return freshPrince.Key
				}, 0.1, 0.01).Should(Equal(conf.ActualFreshnessKey))

				Ω(freshPrince.Timestamp).Should(Equal(timeProvider.Time()))
				Ω(freshPrince.TTL).Should(BeNumerically("==", conf.ActualFreshnessTTL))
			})
		})
	})
})
