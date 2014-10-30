package hm_test

import (
	"errors"
	"time"

	. "github.com/cloudfoundry/hm9000/hm"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Daemon", func() {
	var adapter *fakestoreadapter.FakeStoreAdapter

	BeforeEach(func() {
		adapter = fakestoreadapter.New()
	})

	It("should call the function every PERIOD seconds, unless the function takes *longer* than PERIOD, and it should timeout when the function takes *too* long", func() {
		adapter.OnReleaseNodeChannel = func(releaseNodeChannel chan chan bool) {
			released := <-releaseNodeChannel
			released <- true
		}

		adapter.MaintainNodeStatus <- true

		callTimes := make(chan float64, 4)
		i := 0
		var startTime time.Time
		err := Daemonize("Daemon Test", func() error {
			if i == 0 {
				startTime = time.Now()
			}

			callTimes <- time.Since(startTime).Seconds()
			i += 1
			time.Sleep(time.Duration(i*10) * time.Millisecond)
			return nil
		}, 20*time.Millisecond, 35*time.Millisecond, fakelogger.NewFakeLogger(), adapter)

		Ω(err).Should(Equal(errors.New("Daemon timed out. Aborting!")), "..causes a timeout")

		Eventually(callTimes).Should(HaveLen(4))

		Ω(<-callTimes).Should(BeNumerically("~", 0.0, 0.01), "The first call happens immediately and sleeps for 10 ms")
		Ω(<-callTimes).Should(BeNumerically("~", 0.02, 0.01), "The second call happens after PERIOD and sleeps for 20 ms")
		Ω(<-callTimes).Should(BeNumerically("~", 0.04, 0.01), "The third call happens after PERIOD and sleeps for 30 ms")
		Ω(<-callTimes).Should(BeNumerically(">", 0.0, 0.01), "The fourth call waits for function to finish and happens after 40 ms (> PERIOD) and sleeps for 40 ms which...")
	})

	It("acquires the lock once", func() {
		go Daemonize(
			"ComponentName",
			func() error { return nil },
			20*time.Millisecond,
			35*time.Millisecond,
			fakelogger.NewFakeLogger(),
			adapter,
		)

		Eventually(adapter.GetMaintainedNodeName).Should(Equal("/hm/locks/ComponentName"))
	})

	Context("when the locker fails", func() {
		disaster := errors.New("oh no!")

		BeforeEach(func() {
			adapter.MaintainNodeError = disaster
		})

		It("returns the error", func() {
			err := Daemonize(
				"Daemon Test",
				func() error { Fail("NOPE"); return nil },
				20*time.Millisecond,
				35*time.Millisecond,
				fakelogger.NewFakeLogger(),
				adapter,
			)

			Ω(err).Should(Equal(disaster))
		})
	})

	Context("when the callback times out", func() {
		It("releases the lock", func() {
			didRelease := make(chan bool)

			adapter.OnReleaseNodeChannel = func(releaseNodeChannel chan chan bool) {
				released := <-releaseNodeChannel
				released <- true
				didRelease <- true
			}

			adapter.MaintainNodeStatus <- true

			Daemonize(
				"Daemon Test",
				func() error { time.Sleep(1 * time.Second); return nil },
				20*time.Millisecond,
				35*time.Millisecond,
				fakelogger.NewFakeLogger(),
				adapter,
			)

			Eventually(didRelease).Should(Receive())
		})
	})
})
