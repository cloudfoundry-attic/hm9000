package config

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"runtime"
)

type Config struct {
	HeartbeatTTL               uint64   `json:"heartbeat_ttl_in_seconds"`
	ActualFreshnessTTL         uint64   `json:"actual_freshness_ttl_in_seconds"`
	GracePeriod                int      `json:"grace_period_in_seconds"`
	DesiredStateTTL            uint64   `json:"desired_state_ttl_in_seconds"`
	DesiredFreshnessTTL        uint64   `json:"desired_freshness_ttl_in_seconds"`
	DesiredStateBatchSize      int      `json:"desired_state_batch_size"`
	ActualFreshnessKey         string   `json:"actual_freshness_key"`
	DesiredFreshnessKey        string   `json:"desired_freshness_key"`
	CCAuthMessageBusSubject    string   `json:"cc_auth_message_bus_subject"`
	CCBaseURL                  string   `json:"cc_base_url"`
	StoreURLs                  []string `json:"store_urls"`
	StoreMaxConcurrentRequests int      `json:"store_max_concurrent_requests"`
	SenderNatsStartSubject     string   `json:"sender_nats_start_subject"`
	SenderNatsStopSubject      string   `json:"sender_nats_stop_subject"`

	NATS struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
	} `json:"nats"`
}

func DefaultConfig() (Config, error) {
	_, file, _, _ := runtime.Caller(0)
	pathToJSON := filepath.Clean(filepath.Join(filepath.Dir(file), "default_config.json"))

	return FromFile(pathToJSON)
}

func FromFile(path string) (Config, error) {
	json, err := ioutil.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	return FromJSON(json)
}

func FromJSON(JSON []byte) (Config, error) {
	var config Config
	err := json.Unmarshal(JSON, &config)
	if err == nil {
		return config, nil
	} else {
		return Config{}, err
	}
}
