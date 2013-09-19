package store_test

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"
	"time"
)

var _ = Describe("Freshness", func() {
	var (
		store       Store
		etcdAdapter storeadapter.StoreAdapter
		conf        config.Config
	)

	conf, _ = config.DefaultConfig()

	BeforeEach(func() {
		etcdAdapter = storeadapter.NewETCDStoreAdapter(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err := etcdAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())

		store = NewStore(conf, etcdAdapter)
	})

	Describe("Bumping freshness", func() {
		bumpingFreshness := func(key string, ttl uint64, bump func(store Store, timestamp time.Time) error) {
			var timestamp time.Time

			BeforeEach(func() {
				timestamp = time.Now()
			})

			Context("when the key is missing", func() {
				BeforeEach(func() {
					_, err := etcdAdapter.Get(key)
					Ω(storeadapter.IsKeyNotFoundError(err)).Should(BeTrue())

					err = bump(store, timestamp)
					Ω(err).ShouldNot(HaveOccured())
				})

				It("should create the key with the current timestamp and a TTL", func() {
					value, err := etcdAdapter.Get(key)

					Ω(err).ShouldNot(HaveOccured())

					var freshnessTimestamp models.FreshnessTimestamp
					json.Unmarshal(value.Value, &freshnessTimestamp)

					Ω(freshnessTimestamp.Timestamp).Should(Equal(timestamp.Unix()))
					Ω(value.TTL).Should(BeNumerically("==", ttl-1))
					Ω(value.Key).Should(Equal(key))
				})
			})

			Context("when the key is present", func() {
				BeforeEach(func() {
					err := bump(store, time.Unix(100, 0))
					Ω(err).ShouldNot(HaveOccured())
					err = bump(store, timestamp)
					Ω(err).ShouldNot(HaveOccured())
				})

				It("should bump the key's TTL but not change the timestamp", func() {
					value, err := etcdAdapter.Get(key)

					Ω(err).ShouldNot(HaveOccured())

					Ω(value.TTL).Should(BeNumerically("==", ttl-1))

					var freshnessTimestamp models.FreshnessTimestamp
					json.Unmarshal(value.Value, &freshnessTimestamp)

					Ω(freshnessTimestamp.Timestamp).Should(BeNumerically("==", 100))
					Ω(value.Key).Should(Equal(key))
				})
			})
		}

		Context("the actual state", func() {
			bumpingFreshness(conf.ActualFreshnessKey, conf.ActualFreshnessTTL, Store.BumpActualFreshness)
		})

		Context("the desired state", func() {
			bumpingFreshness(conf.DesiredFreshnessKey, conf.DesiredFreshnessTTL, Store.BumpDesiredFreshness)
		})
	})
})
