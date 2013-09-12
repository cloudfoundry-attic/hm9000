package models

import (
	"encoding/json"
	"time"
)

//Desired app state
type AppState string

const (
	AppStateInvalid AppState = ""
	AppStateStarted AppState = "STARTED"
	AppStateStopped AppState = "STOPPED"
)

type AppPackageState string

const (
	AppPackageStateInvalid AppPackageState = ""
	AppPackageStateFailed  AppPackageState = "FAILED"
	AppPackageStatePending AppPackageState = "PENDING"
	AppPackageStateStaged  AppPackageState = "STAGED"
)

type DesiredAppState struct {
	AppGuid           string          `json:"id"`
	AppVersion        string          `json:"version"`
	NumberOfInstances int             `json:"instances"`
	Memory            int             `json:"memory"`
	State             AppState        `json:"state"`
	PackageState      AppPackageState `json:"package_state"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

func (state DesiredAppState) ToJson() string {
	result, _ := json.Marshal(state)
	return string(result)
}
