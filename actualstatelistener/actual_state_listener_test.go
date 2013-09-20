package actualstatelistener_test

import (
	. "github.com/cloudfoundry/hm9000/actualstatelistener"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"

	. "github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/testhelpers/app"

	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestore"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"
)

var _ = Describe("Actual state listener", func() {
	var (
		app          App
		anotherApp   App
		store        *fakestore.FakeStore
		listener     *ActualStateListener
		timeProvider *faketimeprovider.FakeTimeProvider
		messageBus   *fake_cfmessagebus.FakeMessageBus
		conf         config.Config
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

		store = fakestore.NewFakeStore()
		messageBus = fake_cfmessagebus.NewFakeMessageBus()

		listener = New(conf, messageBus, store, timeProvider, fakelogger.NewFakeLogger())
		listener.Start()
	})

	It("should subscribe to the dea.heartbeat subject", func() {
		Ω(messageBus.Subscriptions).Should(HaveKey("dea.heartbeat"))
		Ω(messageBus.Subscriptions["dea.heartbeat"]).Should(HaveLen(1))
	})

	Context("When it receives a simple heartbeat over the message bus", func() {
		BeforeEach(func() {
			messageBus.Subscriptions["dea.heartbeat"][0].Callback(app.Heartbeat(1, 17).ToJson())
		})

		It("Stores it in the store", func() {
			actual, _ := store.GetActualState()
			Ω(actual).Should(ContainElement(app.GetInstance(0).Heartbeat(17)))
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
			actual, _ := store.GetActualState()
			Ω(actual).Should(ContainElement(app.GetInstance(0).Heartbeat(17)))
			Ω(actual).Should(ContainElement(app.GetInstance(1).Heartbeat(22)))
			Ω(actual).Should(ContainElement(anotherApp.GetInstance(0).Heartbeat(11)))
		})
	})

	Describe("freshness", func() {
		Context("when a heartbeat arrives", func() {
			BeforeEach(func() {
				Ω(store.ActualIsFresh).Should(BeFalse())
				messageBus.Subscriptions["dea.heartbeat"][0].Callback(app.Heartbeat(1, 17).ToJson())
			})

			It("should instruct the store to bump the freshness", func() {
				Ω(store.ActualIsFresh).Should(BeTrue())
				Ω(store.ActualFreshnessTimestamp).Should(Equal(timeProvider.Time()))
			})
		})
	})
})
