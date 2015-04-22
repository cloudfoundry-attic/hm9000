package config_test

import (
	"io/ioutil"
	"time"

	"github.com/cloudfoundry/gosteno"
	. "github.com/cloudfoundry/hm9000/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	configJSON := `
    {
        "heartbeat_period_in_seconds": 11,
        "heartbeat_ttl_in_heartbeats": 3,
        "actual_freshness_ttl_in_heartbeats": 3,
        "grace_period_in_heartbeats": 3,
        "desired_state_ttl_in_heartbeats": 60,
        "desired_freshness_ttl_in_heartbeats": 12,
        "desired_state_batch_size": 500,
        "fetcher_network_timeout_in_seconds": 10,
        "actual_freshness_key": "/actual-fresh",
        "desired_freshness_key": "/desired-fresh",
        "cc_auth_user": "mcat",
        "cc_auth_password": "testing",
        "cc_base_url": "http://127.0.0.1:6001",
        "skip_cert_verify": true,
        "store_schema_version": 1,
        "store_urls": ["http://127.0.0.1:4001"],
        "store_max_concurrent_requests": 30,
        "sender_nats_start_subject": "hm9000.start",
        "sender_nats_stop_subject": "hm9000.stop",
        "sender_message_limit": 60,
        "sender_polling_interval_in_heartbeats": 1,
        "sender_timeout_in_heartbeats": 10,
        "fetcher_polling_interval_in_heartbeats": 6,
        "fetcher_timeout_in_heartbeats": 60,
        "shredder_polling_interval_in_heartbeats": 360,
        "shredder_timeout_in_heartbeats": 6,
        "analyzer_polling_interval_in_heartbeats": 1,
        "analyzer_timeout_in_heartbeats": 10,
        "number_of_crashes_before_backoff_begins": 3,
        "listener_heartbeat_sync_interval_in_milliseconds": 1000,
        "store_heartbeat_cache_refresh_interval_in_milliseconds": 20000,
        "starting_backoff_delay_in_heartbeats": 3,
        "maximum_backoff_delay_in_heartbeats": 96,
        "metrics_server_port": 7879,
        "metrics_server_user": "metrics_server_user",
        "metrics_server_password": "canHazMetrics?",
				"api_server_url": "https://example.com/lol",
        "api_server_port": 5155,
        "api_server_username": "magnet",
        "api_server_password": "orangutan4sale",
        "api_server_address": "0.0.0.0",
        "log_level": "INFO",
        "log_directory": "/some/path",
        "nats": [{
            "host": "127.0.0.1",
            "port": 4222,
            "user": "",
            "password": ""
        }],
				"name": "hm_z1"
    }
    `

	Context("when passed valid JSON", func() {
		It("deserializes", func() {
			config, err := FromJSON([]byte(configJSON))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(config.HeartbeatPeriod).Should(BeNumerically("==", 11))
			Ω(config.HeartbeatTTL()).Should(BeNumerically("==", 33))
			Ω(config.ActualFreshnessTTL()).Should(BeNumerically("==", 33))
			Ω(config.GracePeriod()).Should(BeNumerically("==", 33))
			Ω(config.DesiredFreshnessTTL()).Should(BeNumerically("==", 132))

			Ω(config.SenderPollingInterval().Seconds()).Should(BeNumerically("==", 11))
			Ω(config.SenderTimeout().Seconds()).Should(BeNumerically("==", 110))
			Ω(config.FetcherPollingInterval().Seconds()).Should(BeNumerically("==", 66))
			Ω(config.FetcherTimeout().Seconds()).Should(BeNumerically("==", 660))
			Ω(config.ShredderPollingInterval().Hours()).Should(BeNumerically("==", 1.1))
			Ω(config.ShredderTimeout().Minutes()).Should(BeNumerically("==", 1.1))
			Ω(config.AnalyzerPollingInterval().Seconds()).Should(BeNumerically("==", 11))
			Ω(config.AnalyzerTimeout().Seconds()).Should(BeNumerically("==", 110))

			Ω(config.NumberOfCrashesBeforeBackoffBegins).Should(BeNumerically("==", 3))
			Ω(config.StartingBackoffDelay().Seconds()).Should(BeNumerically("==", 33))
			Ω(config.MaximumBackoffDelay().Seconds()).Should(BeNumerically("==", 1056))

			Ω(config.DesiredStateBatchSize).Should(BeNumerically("==", 500))
			Ω(config.FetcherNetworkTimeout().Seconds()).Should(BeNumerically("==", 10))
			Ω(config.ActualFreshnessKey).Should(Equal("/actual-fresh"))
			Ω(config.DesiredFreshnessKey).Should(Equal("/desired-fresh"))
			Ω(config.CCAuthUser).Should(Equal("mcat"))
			Ω(config.CCAuthPassword).Should(Equal("testing"))
			Ω(config.CCBaseURL).Should(Equal("http://127.0.0.1:6001"))
			Ω(config.SkipSSLVerification).Should(BeTrue())

			Ω(config.ListenerHeartbeatSyncInterval()).Should(Equal(time.Second))
			Ω(config.StoreHeartbeatCacheRefreshInterval()).Should(Equal(20 * time.Second))

			Ω(config.StoreSchemaVersion).Should(Equal(1))
			Ω(config.StoreURLs).Should(Equal([]string{"http://127.0.0.1:4001"}))
			Ω(config.StoreMaxConcurrentRequests).Should(Equal(30))

			Ω(config.SenderNatsStartSubject).Should(Equal("hm9000.start"))
			Ω(config.SenderNatsStopSubject).Should(Equal("hm9000.stop"))
			Ω(config.SenderMessageLimit).Should(Equal(60))

			Ω(config.MetricsServerPort).Should(Equal(7879))
			Ω(config.MetricsServerUser).Should(Equal("metrics_server_user"))
			Ω(config.MetricsServerPassword).Should(Equal("canHazMetrics?"))

			Ω(config.APIServerURL).Should(Equal("https://example.com/lol"))
			Ω(config.APIServerAddress).Should(Equal("0.0.0.0"))
			Ω(config.APIServerPort).Should(Equal(5155))
			Ω(config.APIServerUsername).Should(Equal("magnet"))
			Ω(config.APIServerPassword).Should(Equal("orangutan4sale"))

			Ω(config.NATS[0].Host).Should(Equal("127.0.0.1"))
			Ω(config.NATS[0].Port).Should(Equal(4222))
			Ω(config.NATS[0].User).Should(Equal(""))
			Ω(config.NATS[0].Password).Should(Equal(""))

			Ω(config.LogLevelString).Should(Equal("INFO"))
			Ω(config.LogDirectory).Should(Equal("/some/path"))

			Ω(config.Name).Should(Equal("hm_z1"))
		})
	})

	Describe("LogLevel", func() {
		It("should support INFO and DEBUG", func() {
			config, _ := FromJSON([]byte(configJSON))
			config.LogLevelString = "INFO"
			Ω(config.LogLevel()).Should(Equal(gosteno.LOG_INFO))
			config.LogLevelString = "DEBUG"
			Ω(config.LogLevel()).Should(Equal(gosteno.LOG_DEBUG))
			config.LogLevelString = "Eggplant"
			Ω(config.LogLevel()).Should(Equal(gosteno.LOG_INFO))
		})
	})

	Describe("loading from a file", func() {
		It("should load up the JSON in default_config.json", func() {
			ioutil.WriteFile("/tmp/_test_config.json", []byte(configJSON), 0777)

			config, err := FromFile("/tmp/_test_config.json")
			Ω(err).ShouldNot(HaveOccurred())

			expectedConfig, _ := FromJSON([]byte(configJSON))
			Ω(config).Should(Equal(expectedConfig))
		})
	})

	Context("when passed invalid JSON", func() {
		It("should not deserialize", func() {
			config, err := FromJSON([]byte("¥"))
			Ω(err).Should(HaveOccurred())
			Ω(config).Should(BeNil())
		})
	})
})
