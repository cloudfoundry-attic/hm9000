package actualstatelistener

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"

	. "github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/testhelpers/app"

	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/fakefreshnessmanager"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"
)

var _ = Describe("Actual state listener", func() {
	var (
		app              App
		anotherApp       App
		etcdStoreAdapter storeadapter.StoreAdapter
		listener         *ActualStateListener
		timeProvider     *faketimeprovider.FakeTimeProvider
		freshnessManager *fakefreshnessmanager.FakeFreshnessManager
		messageBus       *fake_cfmessagebus.FakeMessageBus
		conf             config.Config
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())

		timeProvider = &faketimeprovider.FakeTimeProvider{
			TimeToProvide: time.Now(),
		}

		app = NewApp()
		anotherApp = NewApp()

		etcdStoreAdapter = storeadapter.NewETCDStoreAdapter(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err = etcdStoreAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())

		messageBus = fake_cfmessagebus.NewFakeMessageBus()

		freshnessManager = &fakefreshnessmanager.FakeFreshnessManager{}

		listener = New(conf, messageBus, etcdStoreAdapter, freshnessManager, timeProvider, fakelogger.NewFakeLogger())
		listener.Start()
	})

	verifyHeartbeatInStore := func(hb InstanceHeartbeat) {
		storeKey := "/actual/" + hb.InstanceGuid
		var value storeadapter.StoreNode
		var err error

		Eventually(func() interface{} {
			value, err = etcdStoreAdapter.Get(storeKey)
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
					return freshnessManager.Key
				}, 0.1, 0.01).Should(Equal(conf.ActualFreshnessKey))

				Ω(freshnessManager.Timestamp).Should(Equal(timeProvider.Time()))
				Ω(freshnessManager.TTL).Should(BeNumerically("==", conf.ActualFreshnessTTL))
			})
		})
	})
})
