package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"
)

type Config struct {
	HeartbeatTTL          uint64 `json:"heartbeat_ttl_in_seconds"`
	ActualFreshnessTTL    uint64 `json:"actual_freshness_ttl_in_seconds"`
	GracePeriod           int    `json:"grace_period_in_seconds"`
	DesiredStateTTL       uint64 `json:"desired_state_ttl_in_seconds"`
	DesiredFreshnessTTL   uint64 `json:"desired_freshness_ttl_in_seconds"`
	DesiredStateBatchSize int    `json:"desired_state_batch_size"`
	ActualFreshnessKey    string `json:"actual_freshness_key"`
	DesiredFreshnessKey   string `json:"desired_freshness_key"`
	CCBaseURL             string `json:"cc_base_url"`
}

func DefaultConfig() (Config, error) {
	_, file, _, _ := runtime.Caller(0)
	pathToJSON := filepath.Clean(filepath.Join(filepath.Dir(file), "default_config.json"))

	json, err := ioutil.ReadFile(pathToJSON)
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

func ETCD_URL(port int) string {
	//TODO: get rid of this and move it to config as an array of etcd nodes
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}
