package models

import (
	"encoding/json"
	"strconv"
)

//Start and Stop Messages
type StartMessage struct {
	AppGuid        string         `json:"droplet"`
	AppVersion     string         `json:"version"`
	InstanceIndex  int            `json:"instance_index"`
	RunningIndices RunningIndices `json:"running_indices"`
}

type StopMessage struct {
	AppGuid        string         `json:"droplet"`
	AppVersion     string         `json:"version"`
	InstanceGuid   string         `json:"instance_guid"`
	InstanceIndex  int            `json:"instance_index"`
	RunningIndices RunningIndices `json:"running_indices"`
}

type RunningIndices map[string]int

func (indices RunningIndices) IncrementIndex(index int) {
	stringIndex := strconv.Itoa(index)
	_, ok := indices[stringIndex]
	if !ok {
		indices[stringIndex] = 1
		return
	}
	indices[stringIndex] += 1
	return
}

func NewStartMessageFromJSON(encoded []byte) (StartMessage, error) {
	message := StartMessage{}
	err := json.Unmarshal(encoded, &message)
	if err != nil {
		return StartMessage{}, err
	}
	return message, nil
}

func NewStopMessageFromJSON(encoded []byte) (StopMessage, error) {
	message := StopMessage{}
	err := json.Unmarshal(encoded, &message)
	if err != nil {
		return StopMessage{}, err
	}
	return message, nil
}

func (message StartMessage) ToJSON() []byte {
	result, _ := json.Marshal(message)
	return result
}

func (message StopMessage) ToJSON() []byte {
	result, _ := json.Marshal(message)
	return result
}
