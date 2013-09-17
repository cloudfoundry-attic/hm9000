package config

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
)

var _ = Describe("Config", func() {
	configJSON := `
    {
        "heartbeat_ttl_in_seconds": 30,
        "actual_freshness_ttl_in_seconds": 30,
        "grace_period_in_seconds": 30,
        "desired_state_ttl_in_seconds": 600,
        "desired_freshness_ttl_in_seconds": 120,
        "desired_state_batch_size": 500,
        "actual_freshness_key": "/actual-fresh",
        "desired_freshness_key": "/desired-fresh",
        "cc_auth_message_bus_subject": "cloudcontroller.bulk.credentials.default",
        "cc_base_url": "http://127.0.0.1:6001",
        "store_urls": ["http://127.0.0.1:4001"],
        "store_max_concurrent_requests": 100,
        "nats": {
            "host": "127.0.0.1",
            "port": 4222,
            "user": "",
            "password": ""
        }
    }
    `

	Context("when passed valid JSON", func() {
		It("deserializes", func() {
			config, err := FromJSON([]byte(configJSON))
			Ω(err).ShouldNot(HaveOccured())
			Ω(config.HeartbeatTTL).Should(BeNumerically("==", 30))
			Ω(config.ActualFreshnessTTL).Should(BeNumerically("==", 30))
			Ω(config.GracePeriod).Should(BeNumerically("==", 30))
			Ω(config.DesiredStateTTL).Should(BeNumerically("==", 600))
			Ω(config.DesiredFreshnessTTL).Should(BeNumerically("==", 120))
			Ω(config.DesiredStateBatchSize).Should(BeNumerically("==", 500))
			Ω(config.ActualFreshnessKey).Should(Equal("/actual-fresh"))
			Ω(config.DesiredFreshnessKey).Should(Equal("/desired-fresh"))
			Ω(config.CCAuthMessageBusSubject).Should(Equal("cloudcontroller.bulk.credentials.default"))
			Ω(config.CCBaseURL).Should(Equal("http://127.0.0.1:6001"))
			Ω(config.StoreURLs).Should(Equal([]string{"http://127.0.0.1:4001"}))
			Ω(config.StoreMaxConcurrentRequests).Should(Equal(100))

			Ω(config.NATS.Host).Should(Equal("127.0.0.1"))
			Ω(config.NATS.Port).Should(Equal(4222))
			Ω(config.NATS.User).Should(Equal(""))
			Ω(config.NATS.Password).Should(Equal(""))
		})
	})

	Describe("loading up the default config", func() {
		It("should load up the JSON in default_config.json", func() {
			config, err := DefaultConfig()
			Ω(err).ShouldNot(HaveOccured())

			expectedConfig, _ := FromJSON([]byte(configJSON))
			Ω(config).Should(Equal(expectedConfig))
		})
	})

	Describe("loading from a file", func() {
		It("should load up the JSON in default_config.json", func() {
			ioutil.WriteFile("/tmp/_test_config.json", []byte(configJSON), 0777)

			config, err := FromFile("/tmp/_test_config.json")
			Ω(err).ShouldNot(HaveOccured())

			expectedConfig, _ := FromJSON([]byte(configJSON))
			Ω(config).Should(Equal(expectedConfig))
		})
	})

	Context("when passed valid JSON", func() {
		It("deserializes", func() {
			config, err := FromJSON([]byte("¥"))
			Ω(err).Should(HaveOccured())
			Ω(config).Should(BeZero())
		})
	})
})
