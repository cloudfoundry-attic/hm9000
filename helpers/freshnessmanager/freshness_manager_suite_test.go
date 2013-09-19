package freshnessmanager

import (
	"encoding/json"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"

	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/storerunner"

	"testing"
)

var etcdRunner *storerunner.ETCDClusterRunner

func TestFreshnessManager(t *testing.T) {
	etcdRunner = storerunner.NewETCDClusterRunner(5001, 1)
	etcdRunner.Start()

	RegisterFailHandler(Fail)
	RunSpecs(t, "Freshness Manager Suite")

	etcdRunner.Stop()
}

var _ = Describe("Freshness Manager", func() {
	var (
		etcdStoreAdapter storeadapter.StoreAdapter
		manager          FreshnessManager
		key              string
		ttl              uint64
		timestamp        time.Time
	)

	BeforeEach(func() {
		etcdRunner.Reset()

		conf, _ := config.DefaultConfig()
		etcdStoreAdapter = storeadapter.NewETCDStoreAdapter(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err := etcdStoreAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())

		key = "/freshness-key"
		ttl = 20
		timestamp = time.Now()

		manager = NewFreshnessManager(etcdStoreAdapter)
	})

	Describe("Bumping the freshness for a key", func() {

		Context("when the key is missing", func() {
			BeforeEach(func() {
				_, err := etcdStoreAdapter.Get(key)
				Ω(storeadapter.IsKeyNotFoundError(err)).Should(BeTrue())

				manager.Bump(key, ttl, timestamp)
			})

			It("should create the key with the current timestamp and a TTL", func() {
				value, err := etcdStoreAdapter.Get(key)

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
				etcdStoreAdapter.Set([]storeadapter.StoreNode{
					storeadapter.StoreNode{
						Key:   key,
						Value: freshnessTimestamp,
						TTL:   2,
					},
				})

				manager.Bump(key, ttl, timestamp)
			})

			It("should bump the key's TTL but not change the timestamp", func() {
				value, _ := etcdStoreAdapter.Get(key)

				Ω(value.TTL).Should(BeNumerically("==", ttl-1))

				var freshnessTimestamp models.FreshnessTimestamp
				json.Unmarshal(value.Value, &freshnessTimestamp)

				Ω(freshnessTimestamp.Timestamp).Should(BeNumerically("==", 100))
				Ω(value.Key).Should(Equal(key))
			})
		})

	})
})
