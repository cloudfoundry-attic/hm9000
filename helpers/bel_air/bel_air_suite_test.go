package bel_air

import (
	"encoding/json"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"

	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/test_helpers/etcd_runner"

	"testing"
)

var etcdRunner *etcd_runner.ETCDClusterRunner

func TestBootstrap(t *testing.T) {
	etcdRunner = etcd_runner.NewETCDClusterRunner("etcd", 5001, 1)
	etcdRunner.Start()

	RegisterFailHandler(Fail)
	RunSpecs(t, "Bel Air Suite")

	etcdRunner.Stop()
}

var _ = Describe("The Fresh Prince of Bel Air", func() {
	var (
		etcdStore   store.Store
		freshPrince FreshPrince
		key         string
		ttl         uint64
		timestamp   time.Time
	)

	BeforeEach(func() {
		etcdRunner.Reset()

		conf, _ := config.DefaultConfig()
		etcdStore = store.NewETCDStore(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err := etcdStore.Connect()
		Ω(err).ShouldNot(HaveOccured())

		key = "/freshness-key"
		ttl = 20
		timestamp = time.Now()

		freshPrince = NewFreshPrince(etcdStore)
	})

	Describe("Bumping the freshness for a key", func() {

		Context("when the key is missing", func() {
			BeforeEach(func() {
				_, err := etcdStore.Get(key)
				Ω(store.IsKeyNotFoundError(err)).Should(BeTrue())

				freshPrince.Bump(key, ttl, timestamp)
			})

			It("should create the key with the current timestamp and a TTL", func() {
				value, err := etcdStore.Get(key)

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
				freshnessTimestamp, _ := json.Marshal(models.FreshnessTimestamp{Timestamp: 100})
				etcdStore.Set([]store.StoreNode{
					store.StoreNode{
						Key:   key,
						Value: freshnessTimestamp,
						TTL:   2,
					},
				})

				freshPrince.Bump(key, ttl, timestamp)
			})

			It("should bump the key's TTL but not change the timestamp", func() {
				value, _ := etcdStore.Get(key)

				Ω(value.TTL).Should(BeNumerically("==", ttl-1))

				var freshnessTimestamp models.FreshnessTimestamp
				json.Unmarshal(value.Value, &freshnessTimestamp)

				Ω(freshnessTimestamp.Timestamp).Should(BeNumerically("==", 100))
				Ω(value.Key).Should(Equal(key))
			})
		})

	})
})
