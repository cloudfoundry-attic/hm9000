package actualstatelisteners_test

import (
	"errors"

	. "github.com/cloudfoundry/hm9000/actualstatelisteners"
	"github.com/cloudfoundry/hm9000/sender/fakesender"
	"github.com/cloudfoundry/hm9000/store/fakestore"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/clock/fakeclock"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"time"

	. "github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/testhelpers/appfixture"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant/fakemetricsaccountant"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"

	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
)

var _ = Describe("Actual state listener", func() {
	var (
		app               AppFixture
		anotherApp        AppFixture
		dea               DeaFixture
		store             *fakestore.FakeStore
		storeAdapter      *fakestoreadapter.FakeStoreAdapter
		syncer            Syncer
		clock             *fakeclock.FakeClock
		logger            *fakelogger.FakeLogger
		conf              *config.Config
		freshByTime       time.Time
		usageTracker      *fakemetricsaccountant.FakeUsageTracker
		metricsAccountant *fakemetricsaccountant.FakeMetricsAccountant
		syncerProcess     ifrit.Process
		sender            *fakesender.FakeSender
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()

		Expect(err).NotTo(HaveOccurred())

		clock = fakeclock.NewFakeClock(time.Unix(100, 0))

		freshByTime = time.Unix(int64(100+conf.ActualFreshnessTTL()), 0)

		dea = NewDeaFixture()
		app = NewAppFixture()
		anotherApp = NewAppFixture()
		anotherApp.DeaGuid = app.DeaGuid

		storeAdapter = fakestoreadapter.New()
		store = &fakestore.FakeStore{}
		logger = fakelogger.NewFakeLogger()

		usageTracker = &fakemetricsaccountant.FakeUsageTracker{}
		usageTracker.MeasureUsageReturns(0.7, 0)
		metricsAccountant = &fakemetricsaccountant.FakeMetricsAccountant{}
		sender = &fakesender.FakeSender{}
	})

	JustBeforeEach(func() {
		syncer = NewSyncer(logger, conf, store, usageTracker, metricsAccountant, clock, sender)
		syncerProcess = ifrit.Background(syncer)
		Eventually(syncerProcess.Ready()).Should(BeClosed())
	})

	AfterEach(func() {
		ginkgomon.Kill(syncerProcess)
	})

	beat := func() {
		syncer.Heartbeat(app.Heartbeat(1))
	}

	It("To save heartbeats on a timer", func() {
		beat()
		clock.Increment(conf.ListenerHeartbeatSyncInterval())
		Eventually(store.SyncHeartbeatsCallCount).Should(Equal(1))

		beat()
		Consistently(store.SyncHeartbeatsCallCount).Should(Equal(1))

		clock.Increment(conf.ListenerHeartbeatSyncInterval())
		Eventually(store.SyncHeartbeatsCallCount).Should(Equal(2))
	})

	Context("When there are heartbeats", func() {
		receivedHeartbeats := func() {
			Eventually(func() bool {
				count := metricsAccountant.TrackReceivedHeartbeatsCallCount()
				if count > 0 {
					return metricsAccountant.TrackReceivedHeartbeatsArgsForCall(count-1) == 1
				}
				return false
			}).Should(BeTrue())
		}

		JustBeforeEach(func() {
			Expect(store.BumpActualFreshnessCallCount()).To(Equal(0))
			beat()
			clock.Increment(conf.ListenerHeartbeatSyncInterval())
		})

		Context("when there are evacuating heartbeat instances", func() {
			var (
				heartbeats []InstanceHeartbeat
			)

			BeforeEach(func() {
				heartbeats = []InstanceHeartbeat{
					{
						AppGuid:       "guid-1",
						AppVersion:    "version-1",
						InstanceGuid:  "instance-guid-1",
						InstanceIndex: 1,
						State:         InstanceStateEvacuating,
						DeaGuid:       "dea-guid-1",
					},
					{
						AppGuid:       "guid-2",
						AppVersion:    "version-2",
						InstanceGuid:  "instance-guid-2",
						InstanceIndex: 1,
						State:         InstanceStateEvacuating,
						DeaGuid:       "dea-guid-1",
					},
				}
				store.SyncHeartbeatsReturns(heartbeats, nil)
			})

			JustBeforeEach(func() {
				syncer.Heartbeat(&Heartbeat{DeaGuid: "dea-guid-1", InstanceHeartbeats: heartbeats})
			})

			It("send the start messages for all instances", func() {
				Eventually(sender.SendCallCount).Should(Equal(1))

				_, _, startMessages, _ := sender.SendArgsForCall(0)
				Expect(len(startMessages)).To(Equal(2))
				Expect(startMessages[0].AppGuid).To(Equal("guid-1"))
				Expect(startMessages[1].AppGuid).To(Equal("guid-2"))
			})

			It("Sets the message's SkipVerification to true", func() {
				Eventually(sender.SendCallCount).Should(Equal(1))

				_, _, startMessages, _ := sender.SendArgsForCall(0)
				Expect(len(startMessages)).To(Equal(2))
				Expect(startMessages[0].SkipVerification).To(BeTrue())
				Expect(startMessages[1].SkipVerification).To(BeTrue())
			})
		})

		Context("and the SyncHeartbeats completes before the next interval", func() {
			BeforeEach(func() {
				clock := clock
				interval := conf.ListenerHeartbeatSyncInterval()
				store.SyncHeartbeatsStub = func(_ ...*Heartbeat) ([]InstanceHeartbeat, error) {
					clock.Increment(interval - 1)
					return []InstanceHeartbeat{}, nil
				}
			})

			It("Bumps the actual state freshness", func() {
				Eventually(store.BumpActualFreshnessCallCount).Should(Equal(1))
			})

			It("reports the saved heartbeats to the number just saved", func() {
				Eventually(metricsAccountant.TrackSavedHeartbeatsCallCount).Should(Equal(1))
				Expect(metricsAccountant.TrackSavedHeartbeatsArgsForCall(0)).To(Equal(1))

				Expect(metricsAccountant.TrackSavedHeartbeatsArgsForCall(0)).To(Equal(1))
			})

			It("reports the received heartbeats in the interval", func() {
				receivedHeartbeats()
			})
		})

		Context("and the SyncHeartbeats completes after the next interval", func() {
			BeforeEach(func() {
				interval := conf.ListenerHeartbeatSyncInterval()
				clock := clock
				store.SyncHeartbeatsStub = func(_ ...*Heartbeat) ([]InstanceHeartbeat, error) {
					clock.Increment(interval)
					return []InstanceHeartbeat{}, nil
				}
			})

			It("does not bump the actual state freshness", func() {
				Consistently(store.BumpActualFreshnessCallCount).Should(Equal(0))
			})

			It("reports the saved heartbeats", func() {
				Eventually(metricsAccountant.TrackSavedHeartbeatsCallCount).Should(Equal(1))
				Expect(metricsAccountant.TrackSavedHeartbeatsArgsForCall(0)).To(Equal(1))
			})

			It("reports the received heartbeats in the interval it was received", func() {
				// one call per interval, clock incremented twice plus the original loop
				Eventually(metricsAccountant.TrackReceivedHeartbeatsCallCount).Should(Equal(3))

				Expect(metricsAccountant.TrackReceivedHeartbeatsArgsForCall(0)).To(Equal(0))
				Expect(metricsAccountant.TrackReceivedHeartbeatsArgsForCall(1)).To(Equal(1))
				Expect(metricsAccountant.TrackReceivedHeartbeatsArgsForCall(2)).To(Equal(0))
			})
		})

		Context("and SyncHeartbeats had an error", func() {
			BeforeEach(func() {
				store.SyncHeartbeatsReturns([]InstanceHeartbeat{}, errors.New("an error"))
			})

			It("revokes the actual state freshness", func() {
				Eventually(store.RevokeActualFreshnessCallCount).Should(Equal(1))
			})

			It("Does not adjust the saved heartbeats", func() {
				Consistently(metricsAccountant.TrackSavedHeartbeatsCallCount).Should(Equal(0))
			})

			It("Adjusts the received heartbeats", func() {
				receivedHeartbeats()
			})

			It("logs about the failed save", func() {
				Eventually(logger.LoggedSubjects).Should(ContainElement(ContainSubstring("Could not put instance heartbeats in store")))
			})
		})
	})
})
