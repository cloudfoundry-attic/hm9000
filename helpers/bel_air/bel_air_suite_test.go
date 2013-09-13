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

var etcdRunner *etcd_runner.ETCDRunner

func TestBootstrap(t *testing.T) {
	etcdRunner = etcd_runner.NewETCDRunner("etcd", 4001)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Bel Air Suite")

	etcdRunner.StopETCD()
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
		etcdRunner.StopETCD()
		etcdRunner.StartETCD()

		etcdStore = store.NewETCDStore(config.ETCD_URL(4001))
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
				json.Unmarshal([]byte(value.Value), &freshnessTimestamp)

				Ω(freshnessTimestamp.Timestamp).Should(Equal(timestamp.Unix()))
				Ω(value.TTL).Should(BeNumerically("==", ttl-1))
				Ω(value.Key).Should(Equal(key))
			})
		})

		Context("when the key is present", func() {
			BeforeEach(func() {
				freshnessTimestamp, _ := json.Marshal(models.FreshnessTimestamp{Timestamp: 100})
				etcdStore.Set(key, string(freshnessTimestamp), 2)

				freshPrince.Bump(key, ttl, timestamp)
			})

			It("should bump the key's TTL but not change the timestamp", func() {
				value, _ := etcdStore.Get(key)

				Ω(value.TTL).Should(BeNumerically("==", ttl-1))

				var freshnessTimestamp models.FreshnessTimestamp
				json.Unmarshal([]byte(value.Value), &freshnessTimestamp)

				Ω(freshnessTimestamp.Timestamp).Should(BeNumerically("==", 100))
				Ω(value.Key).Should(Equal(key))
			})
		})

	})
})
