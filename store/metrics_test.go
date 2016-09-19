package store_test

import (
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/hm9000/config"
	. "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics", func() {
	var (
		store        Store
		storeAdapter storeadapter.StoreAdapter
		conf         *config.Config
	)

	conf, _ = config.DefaultConfig()

	BeforeEach(func() {
		wpool, err := workpool.NewWorkPool(conf.StoreMaxConcurrentRequests)
		Expect(err).NotTo(HaveOccurred())
		storeAdapter, err = etcdstoreadapter.New(
			&etcdstoreadapter.ETCDOptions{ClusterUrls: etcdRunner.NodeURLS()},
			wpool,
		)
		Expect(err).NotTo(HaveOccurred())
		err = storeAdapter.Connect()
		Expect(err).NotTo(HaveOccurred())

		store = NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
	})

	Describe("Getting and setting a metric", func() {
		BeforeEach(func() {
			err := store.SaveMetric("sprockets", 17)
			Expect(err).NotTo(HaveOccurred())
		})

		It("To store the metric under /metrics", func() {
			_, err := storeAdapter.Get("/hm/v1/metrics/sprockets")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the metric is present", func() {
			It("To return the stored value and no error", func() {
				value, err := store.GetMetric("sprockets")
				Expect(err).NotTo(HaveOccurred())
				Expect(value).To(BeNumerically("==", 17))
			})

			Context("and it is overwritten", func() {
				BeforeEach(func() {
					err := store.SaveMetric("sprockets", 23.5)
					Expect(err).NotTo(HaveOccurred())
				})

				It("To return the new value", func() {
					value, err := store.GetMetric("sprockets")
					Expect(err).NotTo(HaveOccurred())
					Expect(value).To(BeNumerically("==", 23.5))
				})
			})
		})

		Context("when the metric is not present", func() {
			It("To return -1 and an error", func() {
				value, err := store.GetMetric("nonexistent")
				Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
				Expect(value).To(BeNumerically("==", -1))
			})
		})
	})
})
