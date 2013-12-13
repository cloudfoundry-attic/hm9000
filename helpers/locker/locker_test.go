package locker_test

import (
	"fmt"
	. "github.com/cloudfoundry/hm9000/helpers/locker"
	"github.com/coreos/go-etcd/etcd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"runtime"
	"time"
)

var counter = 0

var _ = Describe("Locker", func() {
	var (
		locker               *Locker
		etcdClient           *etcd.Client
		uniqueKeyForThisTest string //avoid collisions between test runs
	)
	BeforeEach(func() {
		etcdClient = etcd.NewClient(etcdRunner.NodeURLS())

		uniqueKeyForThisTest = fmt.Sprintf("analyzer-%d", counter)
		counter++

		locker = New(etcdClient, uniqueKeyForThisTest, 1)
	})

	Context("when passed a TTL of 0", func() {
		It("should be like, no way man", func() {
			locker = New(etcdClient, uniqueKeyForThisTest, 0)
			err := locker.GetAndMaintainLock()
			Ω(err).Should(Equal(NoTTLError))
		})
	})

	Context("when the store is not available", func() {
		BeforeEach(func() {
			etcdRunner.Stop()
		})

		AfterEach(func() {
			etcdRunner.Start()
		})

		It("returns an error", func() {
			err := locker.GetAndMaintainLock()
			Ω(err).Should(Equal(NoStoreError))
		})
	})

	Context("when the lock is available", func() {
		It("should return immediately", func(done Done) {
			err := locker.GetAndMaintainLock()
			Ω(err).ShouldNot(HaveOccurred())
			close(done)
		}, 1.0)

		It("should maintain the lock in the background", func(done Done) {
			locker.GetAndMaintainLock()

			otherLockerDidUnlock := false
			go func() {
				otherLocker := New(etcdClient, uniqueKeyForThisTest, 1)
				otherLocker.GetAndMaintainLock()
				otherLockerDidUnlock = true
			}()
			time.Sleep(3 * time.Second)

			Ω(otherLockerDidUnlock).Should(BeFalse())

			close(done)
		}, 10.0)

		Context("when called again", func() {
			It("should just return immediately", func(done Done) {
				locker.GetAndMaintainLock()
				err := locker.GetAndMaintainLock()
				Ω(err).ShouldNot(HaveOccurred())
				close(done)
			}, 1.0)
		})
	})

	Context("when the lock is unavailable", func() {
		It("should block until the lock becomes available", func(done Done) {
			err := locker.GetAndMaintainLock()
			Ω(err).ShouldNot(HaveOccurred())

			didRun := false
			go func() {
				otherLocker := New(etcdClient, uniqueKeyForThisTest, 1)
				err := otherLocker.GetAndMaintainLock()
				Ω(err).ShouldNot(HaveOccurred())
				didRun = true
			}()

			runtime.Gosched()

			Ω(didRun).Should(BeFalse())
			locker.ReleaseLock()

			Eventually(func() bool { return didRun }, 3).Should(BeTrue())

			close(done)
		}, 10.0)
	})
})
