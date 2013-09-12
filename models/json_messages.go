package models

//Actual app state
//Heartbeats
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
