package config

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
        "cc_base_url": "http://127.0.0.1:6001"
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
			Ω(config.CCBaseURL).Should(Equal("http://127.0.0.1:6001"))
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

	Context("when passed valid JSON", func() {
		It("deserializes", func() {
			config, err := FromJSON([]byte("¥"))
			Ω(err).Should(HaveOccured())
			Ω(config).Should(BeZero())
		})
	})
})
