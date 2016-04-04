package config_test

import (
	"io/ioutil"
	"time"

	. "github.com/cloudfoundry/hm9000/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
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
        "api_server_port": 5155,
        "api_server_username": "magnet",
        "api_server_password": "orangutan4sale",
        "api_server_address": "0.0.0.0",
        "log_level": "INFO",
        "nats": [{
            "host": "127.0.0.1",
            "port": 4222,
            "user": "",
            "password": ""
        }],
        "http_heartbeat": [{
        	"server_address": "0.0.0.0",
        	"port": 5335
        }],
		"dropsonde_port": 12344,
		"consul_cluster": "http://127.0.0.1:8500"
    }
    `

	Context("when passed valid JSON", func() {
		It("deserializes", func() {
			config, err := FromJSON([]byte(configJSON))
			Expect(err).ToNot(HaveOccurred())
			Expect(config.HeartbeatPeriod).To(BeNumerically("==", 11))
			Expect(config.HeartbeatTTL()).To(BeNumerically("==", 33))
			Expect(config.ActualFreshnessTTL()).To(BeNumerically("==", 33))
			Expect(config.GracePeriod()).To(BeNumerically("==", 33))
			Expect(config.DesiredFreshnessTTL()).To(BeNumerically("==", 132))

			Expect(config.SenderPollingInterval().Seconds()).To(BeNumerically("==", 11))
			Expect(config.SenderTimeout().Seconds()).To(BeNumerically("==", 110))
			Expect(config.FetcherPollingInterval().Seconds()).To(BeNumerically("==", 66))
			Expect(config.FetcherTimeout().Seconds()).To(BeNumerically("==", 660))
			Expect(config.ShredderPollingInterval().Hours()).To(BeNumerically("==", 1.1))
			Expect(config.ShredderTimeout().Minutes()).To(BeNumerically("==", 1.1))
			Expect(config.AnalyzerPollingInterval().Seconds()).To(BeNumerically("==", 11))
			Expect(config.AnalyzerTimeout().Seconds()).To(BeNumerically("==", 110))

			Expect(config.NumberOfCrashesBeforeBackoffBegins).To(BeNumerically("==", 3))
			Expect(config.StartingBackoffDelay().Seconds()).To(BeNumerically("==", 33))
			Expect(config.MaximumBackoffDelay().Seconds()).To(BeNumerically("==", 1056))

			Expect(config.DesiredStateBatchSize).To(BeNumerically("==", 500))
			Expect(config.FetcherNetworkTimeout().Seconds()).To(BeNumerically("==", 10))
			Expect(config.ActualFreshnessKey).To(Equal("/actual-fresh"))
			Expect(config.DesiredFreshnessKey).To(Equal("/desired-fresh"))
			Expect(config.CCAuthUser).To(Equal("mcat"))
			Expect(config.CCAuthPassword).To(Equal("testing"))
			Expect(config.CCBaseURL).To(Equal("http://127.0.0.1:6001"))
			Expect(config.SkipSSLVerification).To(BeTrue())

			Expect(config.ListenerHeartbeatSyncInterval()).To(Equal(time.Second))
			Expect(config.StoreHeartbeatCacheRefreshInterval()).To(Equal(20 * time.Second))

			Expect(config.StoreSchemaVersion).To(Equal(1))
			Expect(config.StoreURLs).To(Equal([]string{"http://127.0.0.1:4001"}))
			Expect(config.StoreMaxConcurrentRequests).To(Equal(30))

			Expect(config.SenderNatsStartSubject).To(Equal("hm9000.start"))
			Expect(config.SenderNatsStopSubject).To(Equal("hm9000.stop"))
			Expect(config.SenderMessageLimit).To(Equal(60))

			Expect(config.MetricsServerPort).To(Equal(7879))
			Expect(config.MetricsServerUser).To(Equal("metrics_server_user"))
			Expect(config.MetricsServerPassword).To(Equal("canHazMetrics?"))

			Expect(config.APIServerAddress).To(Equal("0.0.0.0"))
			Expect(config.APIServerPort).To(Equal(5155))
			Expect(config.APIServerUsername).To(Equal("magnet"))
			Expect(config.APIServerPassword).To(Equal("orangutan4sale"))

			Expect(config.NATS[0].Host).To(Equal("127.0.0.1"))
			Expect(config.NATS[0].Port).To(Equal(4222))
			Expect(config.NATS[0].User).To(Equal(""))
			Expect(config.NATS[0].Password).To(Equal(""))

			Expect(config.LogLevelString).To(Equal("INFO"))

			Expect(config.DropsondePort).To(Equal(12344))

			Expect(config.HttpHeartbeatServerAddress).To(Equal("0.0.0.0"))
			Expect(config.HttpHeartbeatPort).To(Equal(5335))

			Expect(config.ConsulCluster).To(Equal("http://127.0.0.1:8500"))
		})
	})

	Describe("LogLevel", func() {
		var config *Config

		BeforeEach(func() {
			config, _ = FromJSON([]byte(configJSON))
		})

		It("should support INFO and DEBUG", func() {
			config.LogLevelString = "INFO"
			logLevel, err := config.LogLevel()
			Expect(err).ToNot(HaveOccurred())
			Expect(logLevel).To(Equal(lager.INFO))

			config.LogLevelString = "DEBUG"
			logLevel, err = config.LogLevel()
			Expect(err).ToNot(HaveOccurred())
			Expect(logLevel).To(Equal(lager.DEBUG))
		})

		It("defaults to INFO if no level is set", func() {
			config.LogLevelString = ""
			logLevel, err := config.LogLevel()
			Expect(err).ToNot(HaveOccurred())
			Expect(logLevel).To(Equal(lager.INFO))
		})

		It("returns an error if there is an unrecognized log level", func() {
			config.LogLevelString = "Eggplant"
			_, err := config.LogLevel()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("loading from a file", func() {
		It("should load up the JSON in default_config.json", func() {
			ioutil.WriteFile("/tmp/_test_config.json", []byte(configJSON), 0777)

			config, err := FromFile("/tmp/_test_config.json")
			Expect(err).ToNot(HaveOccurred())

			expectedConfig, _ := FromJSON([]byte(configJSON))
			Expect(config).To(Equal(expectedConfig))
		})
	})

	Context("when passed invalid JSON", func() {
		It("should not deserialize", func() {
			config, err := FromJSON([]byte("¥"))
			Expect(err).To(HaveOccurred())
			Expect(config).To(BeNil())
		})
	})
})
