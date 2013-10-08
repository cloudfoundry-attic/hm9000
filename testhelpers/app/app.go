package app

import (
	. "github.com/cloudfoundry/hm9000/models"
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

func (app App) CrashedInstanceHeartbeatAtIndex(index int) InstanceHeartbeat {
	return InstanceHeartbeat{
		State:         InstanceStateCrashed,
		CCPartition:   "default",
		AppGuid:       app.AppGuid,
		AppVersion:    app.AppVersion,
		InstanceGuid:  Guid(),
		InstanceIndex: index,
	}
}

func (app App) InstanceAtIndex(index int) Instance {
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

func (app App) DesiredState() DesiredAppState {
	return DesiredAppState{
		AppGuid:           app.AppGuid,
		AppVersion:        app.AppVersion,
		NumberOfInstances: 1,
		Memory:            1024,
		State:             AppStateStarted,
		PackageState:      AppPackageStateStaged,
	}
}

func (instance Instance) Heartbeat() InstanceHeartbeat {
	return InstanceHeartbeat{
		CCPartition:   "default",
		AppGuid:       instance.AppGuid,
		AppVersion:    instance.AppVersion,
		InstanceGuid:  instance.InstanceGuid,
		InstanceIndex: instance.InstanceIndex,
		State:         InstanceStateRunning,
	}
}

func (instance Instance) DropletExited(reason DropletExitedReason) DropletExitedMessage {
	droplet_exited := DropletExitedMessage{
		CCPartition:     "default",
		AppGuid:         instance.AppGuid,
		AppVersion:      instance.AppVersion,
		InstanceGuid:    instance.InstanceGuid,
		InstanceIndex:   instance.InstanceIndex,
		Reason:          reason,
		ExitDescription: "exited",
	}

	return droplet_exited
}

func (app App) Heartbeat(instances int) Heartbeat {
	instanceHeartbeats := make([]InstanceHeartbeat, instances)
	for i := 0; i < instances; i++ {
		instanceHeartbeats[i] = app.InstanceAtIndex(i).Heartbeat()
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
