package store_test

import (
	"code.cloudfoundry.org/workpool"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"
	"time"
)

var _ = Describe("Freshness", func() {
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

	Describe("Bumping freshness", func() {
		bumpingFreshness := func(key string, ttl uint64, bump func(store Store, timestamp time.Time) error) {
			var timestamp time.Time

			BeforeEach(func() {
				timestamp = time.Now()
			})

			Context("when the key is missing", func() {
				BeforeEach(func() {
					_, err := storeAdapter.Get(key)
					Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))

					err = bump(store, timestamp)
					Expect(err).NotTo(HaveOccurred())
				})

				It("To create the key with the current timestamp and a TTL", func() {
					value, err := storeAdapter.Get(key)

					Expect(err).NotTo(HaveOccurred())

					var freshnessTimestamp models.FreshnessTimestamp
					json.Unmarshal(value.Value, &freshnessTimestamp)

					Expect(freshnessTimestamp.Timestamp).To(Equal(timestamp.Unix()))
					Expect(value.TTL).To(BeNumerically("==", ttl))
					Expect(value.Key).To(Equal(key))
				})
			})

			Context("when the key is present", func() {
				BeforeEach(func() {
					err := bump(store, time.Unix(100, 0))
					Expect(err).NotTo(HaveOccurred())
					err = bump(store, timestamp)
					Expect(err).NotTo(HaveOccurred())
				})

				It("To bump the key's TTL but not change the timestamp", func() {
					value, err := storeAdapter.Get(key)

					Expect(err).NotTo(HaveOccurred())

					Expect(value.TTL).To(BeNumerically("==", ttl))

					var freshnessTimestamp models.FreshnessTimestamp
					json.Unmarshal(value.Value, &freshnessTimestamp)

					Expect(freshnessTimestamp.Timestamp).To(BeNumerically("==", 100))
					Expect(value.Key).To(Equal(key))
				})
			})
		}

		Context("the actual state", func() {
			bumpingFreshness("/hm/v1"+conf.ActualFreshnessKey, conf.ActualFreshnessTTL(), Store.BumpActualFreshness)

			Context("revoking actual state freshness", func() {
				BeforeEach(func() {
					store.BumpActualFreshness(time.Unix(100, 0))
				})

				It("To no longer be fresh", func() {
					fresh, err := store.IsActualStateFresh(time.Unix(130, 0))
					Expect(err).NotTo(HaveOccurred())
					Expect(fresh).To(BeTrue())

					store.RevokeActualFreshness()

					fresh, err = store.IsActualStateFresh(time.Unix(130, 0))
					Expect(err).NotTo(HaveOccurred())
					Expect(fresh).To(BeFalse())
				})
			})
		})
	})

	Describe("Verifying the store's freshness", func() {
		Context("when the actual state is not fresh", func() {
			It("To return the appropriate error", func() {
				err := store.VerifyFreshness(time.Unix(100, 0))
				Expect(err).To(Equal(ActualIsNotFreshError))
			})
		})

		Context("when the actual state is fresh", func() {
			It("To not error", func() {
				store.BumpActualFreshness(time.Unix(100, 0))
				err := store.VerifyFreshness(time.Unix(int64(100+conf.ActualFreshnessTTL()), 0))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Checking actual state freshness", func() {
		Context("if the freshness key is not present", func() {
			It("returns that the state is not fresh", func() {
				fresh, err := store.IsActualStateFresh(time.Unix(130, 0))
				Expect(err).NotTo(HaveOccurred())
				Expect(fresh).To(BeFalse())
			})
		})

		Context("if the freshness key is present", func() {
			BeforeEach(func() {
				store.BumpActualFreshness(time.Unix(100, 0))
			})

			Context("if the creation time of the key is outside the last x seconds", func() {
				It("returns that the state is fresh", func() {
					fresh, err := store.IsActualStateFresh(time.Unix(130, 0))
					Expect(err).NotTo(HaveOccurred())
					Expect(fresh).To(BeTrue())
				})
			})

			Context("if the creation time of the key is within the last x seconds", func() {
				It("returns that the state is not fresh", func() {
					fresh, err := store.IsActualStateFresh(time.Unix(129, 0))
					Expect(err).NotTo(HaveOccurred())
					Expect(fresh).To(BeFalse())
				})
			})

			Context("if the freshness key fails to parse", func() {
				BeforeEach(func() {
					storeAdapter.SetMulti([]storeadapter.StoreNode{
						{
							Key:   "/hm/v1/actual-fresh",
							Value: []byte("ß"),
						},
					})
				})

				It("To return an error", func() {
					fresh, err := store.IsActualStateFresh(time.Unix(129, 0))
					Expect(err).To(HaveOccurred())
					Expect(fresh).To(BeFalse())
				})
			})
		})

		Context("when the store returns an error", func() {
			BeforeEach(func() {
				err := storeAdapter.SetMulti([]storeadapter.StoreNode{
					{
						Key:   "/hm/v1/actual-fresh/mwahaha",
						Value: []byte("i'm a directory...."),
					},
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("To return the store's error", func() {
				fresh, err := store.IsActualStateFresh(time.Unix(130, 0))
				Expect(err).To(Equal(storeadapter.ErrorNodeIsDirectory))
				Expect(fresh).To(BeFalse())
			})
		})
	})
})
