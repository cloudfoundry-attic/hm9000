package hm_test

import (
	"errors"
	. "github.com/cloudfoundry/hm9000/hm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Daemon", func() {
	Context("when the function takes less than the period", func() {
		It("should call the function every PERIOD seconds", func() {
			callTimes := []float64{}
			startTime := time.Now()
			i := 0
			err := Daemonize(func() error {
				callTimes = append(callTimes, time.Since(startTime).Seconds())
				time.Sleep(10 * time.Millisecond)
				i += 1
				if i < 3 {
					return nil
				}
				return errors.New("Boom")
			}, 20*time.Millisecond, 50*time.Millisecond)

			Ω(callTimes).Should(HaveLen(3))
			Ω(callTimes[0]).Should(BeNumerically("~", 0, 0.005))
			Ω(callTimes[1]).Should(BeNumerically("~", 0.02, 0.005))
			Ω(callTimes[2]).Should(BeNumerically("~", 0.04, 0.005))
			Ω(err).Should(Equal(errors.New("Boom")))
		})
	})

	Context("when the function takes longer than the period", func() {
		It("should call the function only after the function finishes running", func() {
			callTimes := []float64{}
			startTime := time.Now()
			i := 0
			err := Daemonize(func() error {
				callTimes = append(callTimes, time.Since(startTime).Seconds())
				time.Sleep(30 * time.Millisecond)
				i += 1
				if i < 3 {
					return nil
				}
				return errors.New("Boom")
			}, 20*time.Millisecond, 50*time.Millisecond)

			Ω(callTimes).Should(HaveLen(3))
			Ω(callTimes[0]).Should(BeNumerically("~", 0, 0.005))
			Ω(callTimes[1]).Should(BeNumerically("~", 0.03, 0.005))
			Ω(callTimes[2]).Should(BeNumerically("~", 0.06, 0.005))
			Ω(err).Should(Equal(errors.New("Boom")))
		})
	})

	Context("when the function times out", func() {
		It("should report that it timed out and stop re-running the function", func() {
			callTimes := []float64{}
			startTime := time.Now()
			i := 0
			err := Daemonize(func() error {
				callTimes = append(callTimes, time.Since(startTime).Seconds())
				time.Sleep(60 * time.Millisecond)
				i += 1
				if i < 3 {
					return nil
				}
				return errors.New("Boom")
			}, 20*time.Millisecond, 50*time.Millisecond)

			Ω(callTimes).Should(HaveLen(1))
			Ω(callTimes[0]).Should(BeNumerically("~", 0, 0.005))
			Ω(err).Should(Equal(errors.New("Daemon timed out")))
		})
	})

})
