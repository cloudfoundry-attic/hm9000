package models

import (
	"encoding/json"
	"time"
)

type QueueMessage struct {
	SendOn    int64 `json:"send_on"`
	SentOn    int64 `json:"sent_on"`
	KeepAlive int64 `json:"keep_alive"`
}

type QueueStartMessage struct {
	QueueMessage
	AppGuid        string `json:"droplet"`
	AppVersion     string `json:"version"`
	IndicesToStart []int  `json:"indices"`
}

type QueueStopMessage struct {
	QueueMessage
	InstanceGuid string `json:"instance"`
}

func newQueueMessage(now time.Time, delayInSeconds int64, keepAliveInSeconds int64) QueueMessage {
	return QueueMessage{
		SendOn:    now.Add(time.Duration(delayInSeconds) * time.Second).Unix(),
		SentOn:    0,
		KeepAlive: keepAliveInSeconds,
	}
}

func NewQueueStartMessage(now time.Time, delayInSeconds int64, keepAliveInSeconds int64, appGuid string, appVersion string, indicesToStart []int) QueueStartMessage {
	return QueueStartMessage{
		QueueMessage:   newQueueMessage(now, delayInSeconds, keepAliveInSeconds),
		AppGuid:        appGuid,
		AppVersion:     appVersion,
		IndicesToStart: indicesToStart,
	}
}

func NewQueueStartMessageFromJSON(encoded []byte) (QueueStartMessage, error) {
	message := QueueStartMessage{}
	err := json.Unmarshal(encoded, &message)
	if err != nil {
		return QueueStartMessage{}, err
	}
	return message, nil
}

func (message QueueStartMessage) Key() string {
	return message.AppGuid + "-" + message.AppVersion
}

func (message QueueStartMessage) ToJSON() []byte {
	encoded, _ := json.Marshal(message)
	return encoded
}

func NewQueueStopMessage(now time.Time, delayInSeconds int64, keepAliveInSeconds int64, instanceGuid string) QueueStopMessage {
	return QueueStopMessage{
		QueueMessage: newQueueMessage(now, delayInSeconds, keepAliveInSeconds),
		InstanceGuid: instanceGuid,
	}
}

func NewQueueStopMessageFromJSON(encoded []byte) (QueueStopMessage, error) {
	message := QueueStopMessage{}
	err := json.Unmarshal(encoded, &message)
	if err != nil {
		return QueueStopMessage{}, err
	}
	return message, nil
}

func (message QueueStopMessage) ToJSON() []byte {
	encoded, _ := json.Marshal(message)
	return encoded
}

func (message QueueStopMessage) Key() string {
	return message.InstanceGuid
}
