package config

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"time"

	"github.com/cloudfoundry/gosteno"
)

type Config struct {
	HeartbeatPeriod                 uint64 `json:"heartbeat_period_in_seconds"`
	HeartbeatTTLInHeartbeats        uint64 `json:"heartbeat_ttl_in_heartbeats"`
	ActualFreshnessTTLInHeartbeats  uint64 `json:"actual_freshness_ttl_in_heartbeats"`
	GracePeriodInHeartbeats         uint64 `json:"grace_period_in_heartbeats"`
	DesiredFreshnessTTLInHeartbeats uint64 `json:"desired_freshness_ttl_in_heartbeats"`

	SenderPollingIntervalInHeartbeats   int `json:"sender_polling_interval_in_heartbeats"`
	SenderTimeoutInHeartbeats           int `json:"sender_timeout_in_heartbeats"`
	FetcherPollingIntervalInHeartbeats  int `json:"fetcher_polling_interval_in_heartbeats"`
	FetcherTimeoutInHeartbeats          int `json:"fetcher_timeout_in_heartbeats"`
	ShredderPollingIntervalInHeartbeats int `json:"shredder_polling_interval_in_heartbeats"`
	ShredderTimeoutInHeartbeats         int `json:"shredder_timeout_in_heartbeats"`
	AnalyzerPollingIntervalInHeartbeats int `json:"analyzer_polling_interval_in_heartbeats"`
	AnalyzerTimeoutInHeartbeats         int `json:"analyzer_timeout_in_heartbeats"`

	ListenerHeartbeatSyncIntervalInMilliseconds      int `json:"listener_heartbeat_sync_interval_in_milliseconds"`
	StoreHeartbeatCacheRefreshIntervalInMilliseconds int `json:"store_heartbeat_cache_refresh_interval_in_milliseconds"`

	DesiredStateBatchSize          int    `json:"desired_state_batch_size"`
	FetcherNetworkTimeoutInSeconds int    `json:"fetcher_network_timeout_in_seconds"`
	ActualFreshnessKey             string `json:"actual_freshness_key"`
	DesiredFreshnessKey            string `json:"desired_freshness_key"`
	CCAuthUser                     string `json:"cc_auth_user"`
	CCAuthPassword                 string `json:"cc_auth_password"`
	CCBaseURL                      string `json:"cc_base_url"`
	SkipSSLVerification            bool   `json:"skip_cert_verify"`

	StoreSchemaVersion         int      `json:"store_schema_version"`
	StoreURLs                  []string `json:"store_urls"`
	StoreMaxConcurrentRequests int      `json:"store_max_concurrent_requests"`

	SenderNatsStartSubject string `json:"sender_nats_start_subject"`
	SenderNatsStopSubject  string `json:"sender_nats_stop_subject"`
	SenderMessageLimit     int    `json:"sender_message_limit"`

	NumberOfCrashesBeforeBackoffBegins int `json:"number_of_crashes_before_backoff_begins"`
	StartingBackoffDelayInHeartbeats   int `json:"starting_backoff_delay_in_heartbeats"`
	MaximumBackoffDelayInHeartbeats    int `json:"maximum_backoff_delay_in_heartbeats"`

	MetricsServerPort     int    `json:"metrics_server_port"`
	MetricsServerUser     string `json:"metrics_server_user"`
	MetricsServerPassword string `json:"metrics_server_password"`

	APIServerURL      string `json:"api_server_url"`
	APIServerAddress  string `json:"api_server_address"`
	APIServerPort     int    `json:"api_server_port"`
	APIServerUsername string `json:"api_server_username"`
	APIServerPassword string `json:"api_server_password"`

	LogLevelString string `json:"log_level"`
	LogDirectory   string `json:"log_directory"`

	Name string `json:"name,omitempty"`

	NATS []struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"nats"`
}

func defaults() Config {
	return Config{
		HeartbeatPeriod: 10, // TODO: convert to time.Duration

		HeartbeatTTLInHeartbeats:        3,
		ActualFreshnessTTLInHeartbeats:  3,
		GracePeriodInHeartbeats:         3,
		DesiredFreshnessTTLInHeartbeats: 12,

		StoreMaxConcurrentRequests: 30,

		SenderNatsStartSubject: "hm9000.start",
		SenderNatsStopSubject:  "hm9000.stop",
		SenderMessageLimit:     60, // TODO: unit

		SenderPollingIntervalInHeartbeats:   1,   // why?
		SenderTimeoutInHeartbeats:           10,  // why?
		FetcherPollingIntervalInHeartbeats:  6,   // why?
		FetcherTimeoutInHeartbeats:          60,  // why?
		ShredderPollingIntervalInHeartbeats: 360, // why?
		ShredderTimeoutInHeartbeats:         6,   // why?
		AnalyzerPollingIntervalInHeartbeats: 1,   // why?
		AnalyzerTimeoutInHeartbeats:         10,  // why?

		NumberOfCrashesBeforeBackoffBegins: 3,
		StartingBackoffDelayInHeartbeats:   3,  // why?
		MaximumBackoffDelayInHeartbeats:    96, // why?

		ListenerHeartbeatSyncIntervalInMilliseconds:      1000,  // TODO: convert to time.Duration
		StoreHeartbeatCacheRefreshIntervalInMilliseconds: 20000, // TODO: convert to time.Duration

		MetricsServerPort: 7879,

		APIServerURL:      "https://example.com",
		APIServerAddress:  "0.0.0.0",
		APIServerPort:     5155,
		APIServerUsername: "magnet",
		APIServerPassword: "orangutan4sale",

		LogLevelString: "INFO",

		ActualFreshnessKey:  "/actual-fresh",
		DesiredFreshnessKey: "/desired-fresh",
	}
}

func (conf *Config) HeartbeatTTL() uint64 {
	return conf.HeartbeatTTLInHeartbeats * conf.HeartbeatPeriod
}

func (conf *Config) ActualFreshnessTTL() uint64 {
	return conf.ActualFreshnessTTLInHeartbeats * conf.HeartbeatPeriod
}

func (conf *Config) GracePeriod() int {
	return int(conf.GracePeriodInHeartbeats * conf.HeartbeatPeriod)
}

func (conf *Config) DesiredFreshnessTTL() uint64 {
	return conf.DesiredFreshnessTTLInHeartbeats * conf.HeartbeatPeriod
}

func (conf *Config) FetcherNetworkTimeout() time.Duration {
	return time.Duration(conf.FetcherNetworkTimeoutInSeconds) * time.Second
}

func (conf *Config) SenderPollingInterval() time.Duration {
	return time.Duration(conf.SenderPollingIntervalInHeartbeats*int(conf.HeartbeatPeriod)) * time.Second
}

func (conf *Config) SenderTimeout() time.Duration {
	return time.Duration(conf.SenderTimeoutInHeartbeats*int(conf.HeartbeatPeriod)) * time.Second
}

func (conf *Config) FetcherPollingInterval() time.Duration {
	return time.Duration(conf.FetcherPollingIntervalInHeartbeats*int(conf.HeartbeatPeriod)) * time.Second
}

func (conf *Config) FetcherTimeout() time.Duration {
	return time.Duration(conf.FetcherTimeoutInHeartbeats*int(conf.HeartbeatPeriod)) * time.Second
}

func (conf *Config) ShredderPollingInterval() time.Duration {
	return time.Duration(conf.ShredderPollingIntervalInHeartbeats*int(conf.HeartbeatPeriod)) * time.Second
}

func (conf *Config) ShredderTimeout() time.Duration {
	return time.Duration(conf.ShredderTimeoutInHeartbeats*int(conf.HeartbeatPeriod)) * time.Second
}

func (conf *Config) AnalyzerPollingInterval() time.Duration {
	return time.Duration(conf.AnalyzerPollingIntervalInHeartbeats*int(conf.HeartbeatPeriod)) * time.Second
}

func (conf *Config) AnalyzerTimeout() time.Duration {
	return time.Duration(conf.AnalyzerTimeoutInHeartbeats*int(conf.HeartbeatPeriod)) * time.Second
}

func (conf *Config) StartingBackoffDelay() time.Duration {
	return time.Duration(conf.StartingBackoffDelayInHeartbeats*int(conf.HeartbeatPeriod)) * time.Second
}

func (conf *Config) MaximumBackoffDelay() time.Duration {
	return time.Duration(conf.MaximumBackoffDelayInHeartbeats*int(conf.HeartbeatPeriod)) * time.Second
}

func (conf *Config) ListenerHeartbeatSyncInterval() time.Duration {
	return time.Millisecond * time.Duration(conf.ListenerHeartbeatSyncIntervalInMilliseconds)
}

func (conf *Config) StoreHeartbeatCacheRefreshInterval() time.Duration {
	return time.Millisecond * time.Duration(conf.StoreHeartbeatCacheRefreshIntervalInMilliseconds)
}

func (conf *Config) LogLevel() gosteno.LogLevel {
	switch conf.LogLevelString {
	case "INFO":
		return gosteno.LOG_INFO
	case "DEBUG":
		return gosteno.LOG_DEBUG
	default:
		return gosteno.LOG_INFO
	}
}

func DefaultConfig() (*Config, error) {
	_, file, _, _ := runtime.Caller(0)
	pathToJSON := filepath.Clean(filepath.Join(filepath.Dir(file), "default_config.json"))

	return FromFile(pathToJSON)
}

func FromFile(path string) (*Config, error) {
	json, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return FromJSON(json)
}

func FromJSON(JSON []byte) (*Config, error) {
	config := defaults()
	err := json.Unmarshal(JSON, &config)
	if err == nil {
		return &config, nil
	} else {
		return nil, err
	}
}
