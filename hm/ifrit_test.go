package hm_test

import (
	"errors"
	"time"

	. "github.com/cloudfoundry/hm9000/hm"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("HM9000Component", func() {
	var (
		hmc             *Component
		pollingInterval time.Duration
		timeout         time.Duration
		logger          *fakelogger.FakeLogger
		action          func() error
		actionChan      chan time.Time
	)

	BeforeEach(func() {
		pollingInterval = 100 * time.Millisecond
		timeout = 1 * time.Second
		logger = fakelogger.NewFakeLogger()
		actionChan = make(chan time.Time, 10)
		actionChan := actionChan
		action = func() error {
			actionChan <- time.Now()
			return nil
		}
	})

	JustBeforeEach(func() {
		hmc = NewComponent(
			"component",
			nil,
			pollingInterval,
			timeout,
			logger,
			action,
		)
	})

	It("Exits on receiving the correct signal", func() {
		proc := ifrit.Background(hmc)
		ginkgomon.Kill(proc)
		Eventually(proc.Wait()).Should(Receive(BeNil()))
	})

	It("Exits on receiving the correct signal after the action executes", func() {
		proc := ifrit.Background(hmc)

		Eventually(actionChan).Should(Receive())
		ginkgomon.Kill(proc)
		Eventually(proc.Wait()).Should(Receive(BeNil()))
		Consistently(actionChan).ShouldNot(Receive())
	})

	It("Executes the component action on each polling interval", func() {
		proc := ifrit.Background(hmc)
		var t1, t2, t3 time.Time
		Eventually(actionChan).Should(Receive(&t1))
		Eventually(actionChan).Should(Receive(&t2))
		Eventually(actionChan).Should(Receive(&t3))
		ginkgomon.Kill(proc)

		Expect(t2.Sub(t1)).To(BeNumerically("~", pollingInterval, pollingInterval/2))
		Expect(t3.Sub(t2)).To(BeNumerically("~", pollingInterval, pollingInterval/2))
	})

	Context("When the action returns an error", func() {
		BeforeEach(func() {
			action = func() error {
				actionChan <- time.Now()
				return errors.New("Action failed")
			}
		})

		It("Continues to execute", func() {
			proc := ifrit.Background(hmc)
			Eventually(actionChan).Should(Receive())
			Eventually(actionChan).Should(Receive())
			ginkgomon.Kill(proc)
		})
	})

	Context("when the timeout expires", func() {
		BeforeEach(func() {
			timeout = 10 * time.Millisecond
			action = func() error {
				time.Sleep(2 * timeout)
				return nil
			}
		})

		It("Returns an error", func() {
			proc := ifrit.Background(hmc)
			Eventually(proc.Wait()).Should(Receive(MatchError(Equal("component timed out. Aborting!"))))
		})
	})

})
