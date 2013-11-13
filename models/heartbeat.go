package models

import (
	"encoding/json"
	"strconv"
)

type HeartbeatSummary struct {
	DeaGuid                    string                              `json:"dea"`
	InstanceHeartbeatSummaries map[string]InstanceHeartbeatSummary `json:"summary"`
}

type InstanceHeartbeatSummary struct {
	AppGuid    string        `json:"droplet"`
	AppVersion string        `json:"version"`
	State      InstanceState `json:"state"`
}

type Heartbeat struct {
	DeaGuid            string              `json:"dea"`
	InstanceHeartbeats []InstanceHeartbeat `json:"droplets"`
}

func NewHeartbeatFromJSON(encoded []byte) (Heartbeat, error) {
	var heartbeat Heartbeat
	err := json.Unmarshal(encoded, &heartbeat)
	if err != nil {
		return Heartbeat{}, err
	}
	for i, instanceHeartbeat := range heartbeat.InstanceHeartbeats {
		instanceHeartbeat.DeaGuid = heartbeat.DeaGuid
		heartbeat.InstanceHeartbeats[i] = instanceHeartbeat
	}
	return heartbeat, nil
}

func (heartbeat Heartbeat) ToJSON() []byte {
	encoded, _ := json.Marshal(heartbeat)
	return encoded
}

func (heartbeat Heartbeat) LogDescription() map[string]string {
	var evacuating, running, crashed, starting int
	for _, instanceHeartbeat := range heartbeat.InstanceHeartbeats {
		switch instanceHeartbeat.State {
		case InstanceStateCrashed:
			crashed += 1
		case InstanceStateEvacuating:
			evacuating += 1
		case InstanceStateRunning:
			running += 1
		case InstanceStateStarting:
			starting += 1
		}
	}
	return map[string]string{
		"DEA":        heartbeat.DeaGuid,
		"Evacuating": strconv.Itoa(evacuating),
		"Crashed":    strconv.Itoa(crashed),
		"Running":    strconv.Itoa(running),
		"Starting":   strconv.Itoa(starting),
	}
}

func (heartbeat Heartbeat) HeartbeatSummary() HeartbeatSummary {
	summary := HeartbeatSummary{
		DeaGuid:                    heartbeat.DeaGuid,
		InstanceHeartbeatSummaries: make(map[string]InstanceHeartbeatSummary),
	}

	for _, instanceHeartbeat := range heartbeat.InstanceHeartbeats {
		summary.InstanceHeartbeatSummaries[instanceHeartbeat.InstanceGuid] = InstanceHeartbeatSummary{
			AppGuid:    instanceHeartbeat.AppGuid,
			AppVersion: instanceHeartbeat.AppVersion,
			State:      instanceHeartbeat.State,
		}
	}

	return summary
}

func NewHeartbeatSummaryFromJSON(encoded []byte) (HeartbeatSummary, error) {
	var summary HeartbeatSummary
	err := json.Unmarshal(encoded, &summary)
	if err != nil {
		return HeartbeatSummary{}, err
	}
	return summary, nil
}

func (summary HeartbeatSummary) ToJSON() []byte {
	encoded, _ := json.Marshal(summary)
	return encoded
}

func (summary HeartbeatSummary) ContainsInstanceHeartbeat(instanceHeartbeat InstanceHeartbeat) bool {
	existing, exists := summary.InstanceHeartbeatSummaries[instanceHeartbeat.InstanceGuid]
	return exists && existing.State == instanceHeartbeat.State
}
