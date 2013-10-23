package evacuator_test

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestoreadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/faketimeprovider"
	"github.com/cloudfoundry/yagnats"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"time"

	. "github.com/cloudfoundry/hm9000/evacuator"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Evacuator", func() {
	var (
		evacuator    *Evacuator
		messageBus   *fakeyagnats.FakeYagnats
		storeAdapter *fakestoreadapter.FakeStoreAdapter
		timeProvider *faketimeprovider.FakeTimeProvider

		store storepackage.Store
		app   appfixture.AppFixture
	)

	conf, _ := config.DefaultConfig()

	BeforeEach(func() {
		storeAdapter = fakestoreadapter.New()
		messageBus = fakeyagnats.New()
		store = storepackage.NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
		timeProvider = &faketimeprovider.FakeTimeProvider{
			TimeToProvide: time.Unix(100, 0),
		}

		app = appfixture.NewAppFixture()

		evacuator = New(messageBus, store, timeProvider, fakelogger.NewFakeLogger())
		evacuator.Listen()
	})

	It("should be listening on the message bus for droplet.exited", func() {
		Ω(messageBus.Subscriptions).Should(HaveKey("droplet.exited"))
	})

	Context("when droplet.exited is received", func() {
		Context("when the message is malformed", func() {
			It("does nothing", func() {
				messageBus.Subscriptions["droplet.exited"][0].Callback(&yagnats.Message{
					Payload: "ß",
				})

				pendingStarts, err := store.GetPendingStartMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(pendingStarts).Should(BeEmpty())
			})
		})

		Context("when the reason is DEA_EVACUATION", func() {
			BeforeEach(func() {
				messageBus.Subscriptions["droplet.exited"][0].Callback(&yagnats.Message{
					Payload: string(app.InstanceAtIndex(1).DropletExited(models.DropletExitedReasonDEAEvacuation).ToJSON()),
				})
			})

			It("should put a high priority pending start message (configured to skip verification) into the queue", func() {
				pendingStarts, err := store.GetPendingStartMessages()
				Ω(err).ShouldNot(HaveOccured())

				expectedStartMessage := models.NewPendingStartMessage(timeProvider.Time(), 0, 0, app.AppGuid, app.AppVersion, 1, 2.0)
				expectedStartMessage.SkipVerification = true

				Ω(pendingStarts).Should(ContainElement(EqualPendingStartMessage(expectedStartMessage)))
			})
		})

		Context("when the reason is DEA_SHUTDOWN", func() {
			BeforeEach(func() {
				messageBus.Subscriptions["droplet.exited"][0].Callback(&yagnats.Message{
					Payload: string(app.InstanceAtIndex(1).DropletExited(models.DropletExitedReasonDEAShutdown).ToJSON()),
				})
			})

			It("should put a high priority pending start message (configured to skip verification) into the queue", func() {
				pendingStarts, err := store.GetPendingStartMessages()
				Ω(err).ShouldNot(HaveOccured())

				expectedStartMessage := models.NewPendingStartMessage(timeProvider.Time(), 0, 0, app.AppGuid, app.AppVersion, 1, 2.0)
				expectedStartMessage.SkipVerification = true

				Ω(pendingStarts).Should(ContainElement(EqualPendingStartMessage(expectedStartMessage)))
			})
		})

		Context("when the reason is STOPPED", func() {
			BeforeEach(func() {
				messageBus.Subscriptions["droplet.exited"][0].Callback(&yagnats.Message{
					Payload: string(app.InstanceAtIndex(1).DropletExited(models.DropletExitedReasonStopped).ToJSON()),
				})
			})

			It("should do nothing", func() {
				pendingStarts, err := store.GetPendingStartMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(pendingStarts).Should(BeEmpty())
			})
		})

		Context("when the reason is CRASHED", func() {
			BeforeEach(func() {
				messageBus.Subscriptions["droplet.exited"][0].Callback(&yagnats.Message{
					Payload: string(app.InstanceAtIndex(1).DropletExited(models.DropletExitedReasonCrashed).ToJSON()),
				})
			})

			It("should do nothing", func() {
				pendingStarts, err := store.GetPendingStartMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(pendingStarts).Should(BeEmpty())
			})
		})
	})
})
