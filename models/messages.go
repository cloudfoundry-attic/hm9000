package models

import (
	"encoding/json"
)

//Start and Stop Messages
type StartMessage struct {
	AppGuid        string         `json:"droplet"`
	AppVersion     string         `json:"version"`
	InstanceIndex  int            `json:"instance_index"`
	RunningIndices []RunningIndex `json:"running_indices"`
}

type StopMessage struct {
	AppGuid        string         `json:"droplet"`
	AppVersion     string         `json:"version"`
	InstanceGuid   string         `json:"instance_guid"`
	InstanceIndex  int            `json:"instance_index"`
	RunningIndices []RunningIndex `json:"running_indices"`
}

type RunningIndex struct {
	Index         int `json:"index"`
	NumberRunning int `json:"running"`
}

func (message StartMessage) ToJSON() []byte {
	result, _ := json.Marshal(message)
	return result
}

func (message StopMessage) ToJSON() []byte {
	result, _ := json.Marshal(message)
	return result
}
