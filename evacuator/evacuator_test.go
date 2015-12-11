package evacuator_test

import (
	"time"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"github.com/nats-io/nats"
	"github.com/pivotal-golang/clock/fakeclock"

	. "github.com/cloudfoundry/hm9000/evacuator"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Evacuator", func() {
	var (
		evacuator    *Evacuator
		messageBus   *fakeyagnats.FakeNATSConn
		storeAdapter *fakestoreadapter.FakeStoreAdapter
		clock        *fakeclock.FakeClock

		store storepackage.Store
		app   appfixture.AppFixture
	)

	conf, _ := config.DefaultConfig()

	BeforeEach(func() {
		storeAdapter = fakestoreadapter.New()
		messageBus = fakeyagnats.Connect()
		store = storepackage.NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
		clock = fakeclock.NewFakeClock(time.Unix(100, 0))

		app = appfixture.NewAppFixture()

		evacuator = New(messageBus, store, clock, conf, fakelogger.NewFakeLogger())
		evacuator.Listen()
	})

	It("should be listening on the message bus for droplet.exited", func() {
		Expect(messageBus.SubjectCallbacks("droplet.exited")).NotTo(BeNil())
	})

	Context("when droplet.exited is received", func() {
		Context("when the message is malformed", func() {
			It("does nothing", func() {
				messageBus.SubjectCallbacks("droplet.exited")[0](&nats.Msg{
					Data: []byte("ß"),
				})

				pendingStarts, err := store.GetPendingStartMessages()
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingStarts).To(BeEmpty())
			})
		})

		Context("when the reason is DEA_EVACUATION", func() {
			BeforeEach(func() {
				messageBus.SubjectCallbacks("droplet.exited")[0](&nats.Msg{
					Data: app.InstanceAtIndex(1).DropletExited(models.DropletExitedReasonDEAEvacuation).ToJSON(),
				})
			})

			It("should put a high priority pending start message (configured to skip verification) into the queue", func() {
				pendingStarts, err := store.GetPendingStartMessages()
				Expect(err).NotTo(HaveOccurred())

				expectedStartMessage := models.NewPendingStartMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, 1, 2.0, models.PendingStartMessageReasonEvacuating)
				expectedStartMessage.SkipVerification = true

				Expect(pendingStarts).To(ContainElement(EqualPendingStartMessage(expectedStartMessage)))
			})
		})

		Context("when the reason is DEA_SHUTDOWN", func() {
			BeforeEach(func() {
				messageBus.SubjectCallbacks("droplet.exited")[0](&nats.Msg{
					Data: app.InstanceAtIndex(1).DropletExited(models.DropletExitedReasonDEAShutdown).ToJSON(),
				})
			})

			It("should put a high priority pending start message (configured to skip verification) into the queue", func() {
				pendingStarts, err := store.GetPendingStartMessages()
				Expect(err).NotTo(HaveOccurred())

				expectedStartMessage := models.NewPendingStartMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, 1, 2.0, models.PendingStartMessageReasonEvacuating)
				expectedStartMessage.SkipVerification = true

				Expect(pendingStarts).To(ContainElement(EqualPendingStartMessage(expectedStartMessage)))
			})
		})

		Context("when the reason is STOPPED", func() {
			BeforeEach(func() {
				messageBus.SubjectCallbacks("droplet.exited")[0](&nats.Msg{
					Data: app.InstanceAtIndex(1).DropletExited(models.DropletExitedReasonStopped).ToJSON(),
				})
			})

			It("should do nothing", func() {
				pendingStarts, err := store.GetPendingStartMessages()
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingStarts).To(BeEmpty())
			})
		})

		Context("when the reason is CRASHED", func() {
			BeforeEach(func() {
				messageBus.SubjectCallbacks("droplet.exited")[0](&nats.Msg{
					Data: app.InstanceAtIndex(1).DropletExited(models.DropletExitedReasonCrashed).ToJSON(),
				})
			})

			It("should do nothing", func() {
				pendingStarts, err := store.GetPendingStartMessages()
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingStarts).To(BeEmpty())
			})
		})
	})
})
