package app

import (
	. "github.com/cloudfoundry/hm9000/models"

	"time"
)

type App struct {
	AppGuid    string
	AppVersion string

	instances map[int]Instance
	deaGuid   string
}

type Instance struct {
	InstanceGuid  string
	InstanceIndex int
	AppGuid       string
	AppVersion    string
}

func NewApp() App {
	return newAppForDeaGuid(Guid())
}

func newAppForDeaGuid(deaGuid string) App {
	return App{
		AppGuid:    Guid(),
		AppVersion: Guid(),
		instances:  make(map[int]Instance, 0),
		deaGuid:    deaGuid,
	}
}

func (app App) GetInstance(index int) Instance {
	_, ok := app.instances[index]
	if !ok {
		app.instances[index] = Instance{
			InstanceGuid:  Guid(),
			InstanceIndex: index,
			AppGuid:       app.AppGuid,
			AppVersion:    app.AppVersion,
		}
	}

	return app.instances[index]
}

func (app App) DesiredState(updatedAt int64) DesiredAppState {
	return DesiredAppState{
		AppGuid:           app.AppGuid,
		AppVersion:        app.AppVersion,
		NumberOfInstances: 1,
		Memory:            1024,
		State:             AppStateStarted,
		PackageState:      AppPackageStateStaged,
		UpdatedAt:         time.Unix(updatedAt, 0),
	}
}

func (app App) DesiredStateArr(updatedAt int64) []DesiredAppState {
	return []DesiredAppState{
		app.DesiredState(updatedAt),
	}
}

func (instance Instance) Heartbeat(timestamp int64) InstanceHeartbeat {
	return InstanceHeartbeat{
		CCPartition:    "default",
		AppGuid:        instance.AppGuid,
		AppVersion:     instance.AppVersion,
		InstanceGuid:   instance.InstanceGuid,
		InstanceIndex:  instance.InstanceIndex,
		State:          InstanceStateRunning,
		StateTimestamp: float64(timestamp),
	}
}

func (instance Instance) DropletExited(reason DropletExitedReason, crashTimestamp int64) DropletExitedMessage {
	droplet_exited := DropletExitedMessage{
		CCPartition:     "default",
		AppGuid:         instance.AppGuid,
		AppVersion:      instance.AppVersion,
		InstanceGuid:    instance.InstanceGuid,
		InstanceIndex:   instance.InstanceIndex,
		Reason:          reason,
		ExitDescription: "exited",
	}

	if reason == DropletExitedReasonCrashed {
		droplet_exited.ExitDescription = "crashed"
		droplet_exited.ExitStatusCode = 1
		droplet_exited.CrashTimestamp = crashTimestamp
	}

	return droplet_exited
}

func (app App) Heartbeat(instances int, timestamp int64) Heartbeat {
	instanceHeartbeats := make([]InstanceHeartbeat, instances)
	for i := 0; i < instances; i++ {
		instanceHeartbeats[i] = app.GetInstance(i).Heartbeat(timestamp)
	}

	return Heartbeat{
		DeaGuid:            app.deaGuid,
		InstanceHeartbeats: instanceHeartbeats,
	}
}

func (app App) DropletUpdated() DropletUpdatedMessage {
	return DropletUpdatedMessage{
		AppGuid: app.AppGuid,
	}
}
