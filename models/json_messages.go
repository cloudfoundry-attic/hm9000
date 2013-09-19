package models

//Start and Stop Messages
type StartMessage struct {
	AppGuid                   string         `json:"droplet"`
	AppVersion                string         `json:"version"`
	LastUpdated               int64          `json:"last_updated"`
	IndicesToStart            []int          `json:"indices"`
	RunningInstancesByVersion map[string]int `json:"running"`
	Flapping                  bool           `json:"flapping"`
}

type StopMessage struct {
	AppGuid                   string            `json:"droplet"`
	AppVersion                string            `json:"version"`
	LastUpdated               int64             `json:"last_updated"`
	InstancesToVersion        map[string]string `json:"instances"`
	RunningInstancesByVersion map[string]int    `json:"running"`
}

// Queue Messages

type QueueStartMessage struct {
	AppGuid        string `json:"droplet"`
	AppVersion     string `json:"version"`
	IndicesToStart []int  `json:"indices"`
}

type QueueStopMessage struct {
	InstanceGuid string `json:"instance"`
}

//Droplet Exited
type DropletExitedReason string

const (
	DropletExitedReasonInvalid       DropletExitedReason = ""
	DropletExitedReasonStopped       DropletExitedReason = "STOPPED"
	DropletExitedReasonCrashed       DropletExitedReason = "CRASHED"
	DropletExitedReasonDEAShutdown   DropletExitedReason = "DEA_SHUTDOWN"
	DropletExitedReasonDEAEvacuation DropletExitedReason = "DEA_EVACUATION"
)

type DropletExitedMessage struct {
	CCPartition     string              `json:"cc_partition"`
	AppGuid         string              `json:"droplet"`
	AppVersion      string              `json:"version"`
	InstanceGuid    string              `json:"instance"`
	InstanceIndex   int                 `json:"index"`
	Reason          DropletExitedReason `json:"reason"`
	ExitStatusCode  int                 `json:"exit_status"`
	ExitDescription string              `json:"exit_description"`
	CrashTimestamp  int64               `json:"crash_timestamp,omitempty"`
}

type DropletUpdatedMessage struct {
	AppGuid string `json:"droplet"`
}

//Freshness Timestamp

type FreshnessTimestamp struct {
	Timestamp int64 `json:"timestamp"`
}
