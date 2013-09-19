package models

import (
	"encoding/json"
)

type InstanceState string

const (
	InstanceStateInvalid  InstanceState = ""
	InstanceStateStarting InstanceState = "STARTING"
	InstanceStateRunning  InstanceState = "RUNNING"
	InstanceStateCrashed  InstanceState = "CRASHED"
)

type Heartbeat struct {
	DeaGuid            string              `json:"dea"`
	InstanceHeartbeats []InstanceHeartbeat `json:"droplets"`
}

type InstanceHeartbeat struct {
	CCPartition    string        `json:"cc_partition"`
	AppGuid        string        `json:"droplet"`
	AppVersion     string        `json:"version"`
	InstanceGuid   string        `json:"instance"`
	InstanceIndex  int           `json:"index"`
	State          InstanceState `json:"state"`
	StateTimestamp float64       `json:"state_timestamp"`
}

func NewHeartbeatFromJSON(encoded []byte) (Heartbeat, error) {
	var heartbeat Heartbeat
	err := json.Unmarshal(encoded, &heartbeat)
	if err != nil {
		return Heartbeat{}, err
	}
	return heartbeat, nil
}

func (heartbeat Heartbeat) ToJson() []byte {
	encoded, _ := json.Marshal(heartbeat)
	return encoded
}

func NewInstanceHeartbeatFromJSON(encoded []byte) (InstanceHeartbeat, error) {
	var instance InstanceHeartbeat
	err := json.Unmarshal(encoded, &instance)
	if err != nil {
		return InstanceHeartbeat{}, err
	}
	return instance, nil
}

func (instance InstanceHeartbeat) ToJson() []byte {
	encoded, _ := json.Marshal(instance)
	return encoded
}

func (instance InstanceHeartbeat) StoreKey() string {
	return instance.InstanceGuid
}
