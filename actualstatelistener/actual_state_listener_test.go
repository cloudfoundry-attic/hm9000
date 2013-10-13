package actualstatelistener_test

import (
	"errors"
	. "github.com/cloudfoundry/hm9000/actualstatelistener"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"

	. "github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/testhelpers/appfixture"

	"github.com/cloudfoundry/go_cfmessagebus/fake_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestore"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"
)

var _ = Describe("Actual state listener", func() {
	var (
		app          AppFixture
		anotherApp   AppFixture
		store        *fakestore.FakeStore
		listener     *ActualStateListener
		timeProvider *faketimeprovider.FakeTimeProvider
		messageBus   *fake_cfmessagebus.FakeMessageBus
		logger       *fakelogger.FakeLogger
		conf         config.Config
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())

		timeProvider = &faketimeprovider.FakeTimeProvider{
			TimeToProvide: time.Now(),
		}

		app = NewAppFixture()
		anotherApp = NewAppFixture()

		store = fakestore.NewFakeStore()
		messageBus = fake_cfmessagebus.NewFakeMessageBus()
		logger = fakelogger.NewFakeLogger()

		listener = New(conf, messageBus, store, timeProvider, logger)
		listener.Start()
	})

	It("should subscribe to the dea.heartbeat subject", func() {
		Ω(messageBus.Subscriptions).Should(HaveKey("dea.heartbeat"))
		Ω(messageBus.Subscriptions["dea.heartbeat"]).Should(HaveLen(1))
	})

	It("should subscribe to the dea.advertise subject", func() {
		Ω(messageBus.Subscriptions).Should(HaveKey("dea.advertise"))
		Ω(messageBus.Subscriptions["dea.advertise"]).Should(HaveLen(1))
	})

	Context("When it receives a dea advertisement over the message bus", func() {
		BeforeEach(func() {
			Ω(store.ActualFreshnessTimestamp).Should(BeZero())
			messageBus.Subscriptions["dea.advertise"][0].Callback([]byte("doesn't matter"))
		})

		It("Bumps the actual state freshness", func() {
			Ω(store.ActualFreshnessTimestamp).Should(Equal(timeProvider.Time()))
		})
	})

	Context("When it receives a simple heartbeat over the message bus", func() {
		BeforeEach(func() {
			messageBus.Subscriptions["dea.heartbeat"][0].Callback(app.Heartbeat(1).ToJSON())
		})

		It("Stores it in the store", func() {
			actual, _ := store.GetActualState()
			Ω(actual).Should(ContainElement(app.InstanceAtIndex(0).Heartbeat()))
		})
	})

	Context("When it receives a complex heartbeat with multiple apps and instances", func() {
		JustBeforeEach(func() {
			Ω(store.ActualFreshnessTimestamp).Should(BeZero())

			heartbeat := Heartbeat{
				DeaGuid: Guid(),
				InstanceHeartbeats: []InstanceHeartbeat{
					app.InstanceAtIndex(0).Heartbeat(),
					app.InstanceAtIndex(1).Heartbeat(),
					anotherApp.InstanceAtIndex(0).Heartbeat(),
				},
			}

			messageBus.Subscriptions["dea.heartbeat"][0].Callback(heartbeat.ToJSON())
		})

		It("Stores it in the store", func() {
			actual, _ := store.GetActualState()
			Ω(actual).Should(ContainElement(app.InstanceAtIndex(0).Heartbeat()))
			Ω(actual).Should(ContainElement(app.InstanceAtIndex(1).Heartbeat()))
			Ω(actual).Should(ContainElement(anotherApp.InstanceAtIndex(0).Heartbeat()))
		})

		Context("when the save succeeds", func() {
			It("bumps the freshness", func() {
				Ω(store.ActualFreshnessTimestamp).Should(Equal(timeProvider.Time()))
			})

			Context("when the freshness bump fails", func() {
				BeforeEach(func() {
					store.BumpActualFreshnessError = errors.New("oops")
				})

				It("logs about the failed freshness bump", func() {
					Ω(logger.LoggedSubjects).Should(ContainElement("Could not update actual freshness"))
				})
			})
		})

		Context("when the save fails", func() {
			BeforeEach(func() {
				store.SaveActualStateError = errors.New("oops")
			})

			It("does not bump the freshness", func() {
				Ω(store.ActualFreshnessTimestamp).Should(BeZero())
			})

			It("logs about the failed save", func() {
				Ω(logger.LoggedSubjects).Should(ContainElement(ContainSubstring("Could not put instance heartbeats in store")))
			})
		})
	})

	Context("When it fails to parse the heartbeat message", func() {
		BeforeEach(func() {
			messageBus.Subscriptions["dea.heartbeat"][0].Callback([]byte("ß"))
		})

		It("Stores nothing in the store", func() {
			actual, _ := store.GetActualState()
			Ω(actual).Should(BeEmpty())
		})

		It("does not bump the freshness", func() {
			Ω(store.ActualFreshnessTimestamp).Should(BeZero())
		})

		It("logs about the failed parse", func() {
			Ω(logger.LoggedSubjects).Should(ContainElement("Could not unmarshal heartbeat"))
		})
	})
})
