package evacuator_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant/fakemetricsaccountant"
	"github.com/cloudfoundry/hm9000/models"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/cloudfoundry/hm9000/testhelpers/custommatchers"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	"github.com/nats-io/nats"
	"code.cloudfoundry.org/clock/fakeclock"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/cloudfoundry/hm9000/evacuator"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Evacuator", func() {
	var (
		evacuator             *Evacuator
		messageBus            *fakeyagnats.FakeNATSConn
		storeAdapter          *fakestoreadapter.FakeStoreAdapter
		metricsAccountant     *fakemetricsaccountant.FakeMetricsAccountant
		clock                 *fakeclock.FakeClock
		server                *httptest.Server
		receivedStartMessages []models.StartMessage
		httpError             error

		store            storepackage.Store
		app              appfixture.AppFixture
		evacuatorProcess ifrit.Process
	)

	conf, _ := config.DefaultConfig()

	BeforeEach(func() {
		storeAdapter = fakestoreadapter.New()
		messageBus = fakeyagnats.Connect()
		store = storepackage.NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
		metricsAccountant = &fakemetricsaccountant.FakeMetricsAccountant{}
		clock = fakeclock.NewFakeClock(time.Unix(100, 0))
		receivedStartMessages = []models.StartMessage{}
		app = appfixture.NewAppFixture()
		store.BumpActualFreshness(time.Unix(10, 0))

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := ioutil.ReadAll(r.Body)
			r.Body.Close()
			Expect(err).ToNot(HaveOccurred())

			var startMessage models.StartMessage
			startMessage, httpError = models.NewStartMessageFromJSON(b)
			Expect(err).ToNot(HaveOccurred())
			receivedStartMessages = append(receivedStartMessages, startMessage)

			w.WriteHeader(200)
		}))
		conf.CCInternalURL = server.URL
	})

	JustBeforeEach(func() {
		evacuator = New(messageBus, store, clock, metricsAccountant, conf, fakelogger.NewFakeLogger())
		evacuatorProcess = ifrit.Background(evacuator)
	})

	AfterEach(func() {
		ginkgomon.Kill(evacuatorProcess)
		server.Close()
	})

	Context("when the subscription fails", func() {
		BeforeEach(func() {
			messageBus.WhenSubscribing("droplet.exited",
				func(nats.MsgHandler) error { return errors.New("an error") })
		})

		It("fails", func() {
			Eventually(evacuatorProcess.Wait()).Should(Receive(HaveOccurred()))
		})
	})

	Context("when the subscription succeeds", func() {
		JustBeforeEach(func() {
			Eventually(evacuatorProcess.Ready()).Should(BeClosed())
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
				JustBeforeEach(func() {
					messageBus.SubjectCallbacks("droplet.exited")[0](&nats.Msg{
						Data: app.InstanceAtIndex(1).DropletExited(models.DropletExitedReasonDEAEvacuation).ToJSON(),
					})
				})

				It("should put a high priority pending start message (configured to skip verification) into the queue", func() {
					pendingStarts, err := store.GetPendingStartMessages()
					Expect(err).NotTo(HaveOccurred())

					expectedStartMessage := models.NewPendingStartMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, 1, 2.0, models.PendingStartMessageReasonEvacuating)
					expectedStartMessage.SkipVerification = true
					expectedStartMessage.SentOn = expectedStartMessage.SendOn

					Expect(pendingStarts).To(ContainElement(EqualPendingStartMessage(expectedStartMessage)))
				})

				It("sends the message", func() {
					pendingStarts, err := store.GetPendingStartMessages()
					Expect(err).NotTo(HaveOccurred())

					Expect(pendingStarts).To(HaveLen(1))

					var expectedStartMessage models.PendingStartMessage
					for _, msg := range pendingStarts {
						expectedStartMessage = msg
					}

					Expect(receivedStartMessages).To(HaveLen(1))
					Expect(receivedStartMessages[0]).To(Equal(models.StartMessage{
						AppGuid:       app.AppGuid,
						AppVersion:    app.AppVersion,
						InstanceIndex: 1,
						MessageId:     expectedStartMessage.MessageId,
					}))
				})
			})

			Context("when the reason is DEA_SHUTDOWN", func() {
				JustBeforeEach(func() {
					messageBus.SubjectCallbacks("droplet.exited")[0](&nats.Msg{
						Data: app.InstanceAtIndex(1).DropletExited(models.DropletExitedReasonDEAShutdown).ToJSON(),
					})
				})

				It("should put a high priority pending start message (configured to skip verification) into the queue", func() {
					pendingStarts, err := store.GetPendingStartMessages()
					Expect(err).NotTo(HaveOccurred())

					expectedStartMessage := models.NewPendingStartMessage(clock.Now(), 0, conf.GracePeriod(), app.AppGuid, app.AppVersion, 1, 2.0, models.PendingStartMessageReasonEvacuating)
					expectedStartMessage.SkipVerification = true
					expectedStartMessage.SentOn = expectedStartMessage.SendOn

					Expect(pendingStarts).To(ContainElement(EqualPendingStartMessage(expectedStartMessage)))
				})

				It("sends the message", func() {

					pendingStarts, err := store.GetPendingStartMessages()
					Expect(err).NotTo(HaveOccurred())

					Expect(pendingStarts).To(HaveLen(1))

					var expectedStartMessage models.PendingStartMessage
					for _, msg := range pendingStarts {
						expectedStartMessage = msg
					}

					Expect(receivedStartMessages).To(HaveLen(1))
					Expect(receivedStartMessages[0]).To(Equal(models.StartMessage{
						AppGuid:       app.AppGuid,
						AppVersion:    app.AppVersion,
						InstanceIndex: 1,
						MessageId:     expectedStartMessage.MessageId,
					}))
				})
			})

			Context("when the reason is STOPPED", func() {
				JustBeforeEach(func() {
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
				JustBeforeEach(func() {
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
})
