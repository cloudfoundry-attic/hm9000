package actual_state_listener

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"
	"strconv"
	"time"

	. "github.com/cloudfoundry/hm9000/mcat/app"
	. "github.com/cloudfoundry/hm9000/models"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers"
	"github.com/cloudfoundry/hm9000/store"
)

var _ = Describe("Actual state listener", func() {
	var app App
	var anotherApp App
	var etcdStore store.Store

	var listener *ActualStateListener
	var timeProvider *helpers.FakeTimeProvider

	BeforeEach(func() {
		timeProvider = &helpers.FakeTimeProvider{
			TimeToProvide: time.Now(),
		}

		app = NewApp()
		anotherApp = NewApp()

		etcdStore = store.NewETCDStore(config.ETCD_URL)
		err := etcdStore.Connect()
		Ω(err).ShouldNot(HaveOccured())

		listener = NewActualStateListener(natsRunner.MessageBus, etcdStore, timeProvider)
		listener.Start()
	})

	verifyHeartbeatInStore := func(hb InstanceHeartbeat) {
		storeKey := "/actual/" + hb.AppGuid + "-" + hb.AppVersion + "/" + strconv.Itoa(hb.InstanceIndex) + "/" + hb.InstanceGuid
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
		Context("when /actual/fresh is missing", func() {
			BeforeEach(func() {
				_, err := etcdStore.Get(config.ACTUAL_FRESHNESS_KEY)
				Ω(store.IsKeyNotFoundError(err)).Should(BeTrue())
			})

			Context("and a heartbeat arrives", func() {
				BeforeEach(func() {
					messagePublisher.PublishHeartbeat(app.Heartbeat(1, 17))
				})

				It("should create /actual/fresh with the current timestamp and a TTL", func() {
					var value store.StoreNode
					var err error

					Eventually(func() interface{} {
						value, err = etcdStore.Get(config.ACTUAL_FRESHNESS_KEY)
						return err
					}, 0.1, 0.01).ShouldNot(HaveOccured())

					var timestamp FreshnessTimestamp
					json.Unmarshal([]byte(value.Value), &timestamp)

					Ω(timestamp.Timestamp).Should(Equal(timeProvider.Time().Unix()))
					Ω(value.TTL).Should(BeNumerically("==", config.ACTUAL_FRESHNESS_TTL-1))
					Ω(value.Key).Should(Equal(config.ACTUAL_FRESHNESS_KEY))
				})
			})
		})

		Context("when /actual/fresh is present", func() {
			BeforeEach(func() {
				timestamp, _ := json.Marshal(FreshnessTimestamp{Timestamp: 100})
				etcdStore.Set(config.ACTUAL_FRESHNESS_KEY, string(timestamp), 2)
			})

			Context("and a heartbeat arrives", func() {
				BeforeEach(func() {
					messagePublisher.PublishHeartbeat(app.Heartbeat(1, 17))
				})

				It("should bump /actual/fresh's TTL but not change the timestamp", func() {
					var value store.StoreNode

					Eventually(func() interface{} {
						value, _ = etcdStore.Get(config.ACTUAL_FRESHNESS_KEY)
						return value.TTL
					}, 0.1, 0.01).Should(BeNumerically("==", config.ACTUAL_FRESHNESS_TTL-1))

					var timestamp FreshnessTimestamp
					json.Unmarshal([]byte(value.Value), &timestamp)

					Ω(timestamp.Timestamp).Should(BeNumerically("==", 100))
					Ω(value.Key).Should(Equal(config.ACTUAL_FRESHNESS_KEY))
				})
			})
		})
	})
})
