package sender

import (
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
)

type Sender struct {
	store      store.Store
	storecache *storecache.StoreCache

	messageBus   cfmessagebus.MessageBus
	timeProvider timeprovider.TimeProvider
}

func New(store store.Store, messageBus cfmessagebus.MessageBus, timeProvider timeprovider.TimeProvider) *Sender {
	return &Sender{
		store:        store,
		messageBus:   messageBus,
		timeProvider: timeProvider,
		storecache:   storecache.New(store),
	}
}

func (sender *Sender) Send() error {
	startMessages, err := sender.store.GetQueueStartMessages()
	if err != nil {
		return err
	}

	stopMessages, err := sender.store.GetQueueStopMessages()
	if err != nil {
		return err
	}

	err = sender.storecache.Load()
	if err != nil {
		return err
	}

	err = sender.sendStartMessages(startMessages)
	if err != nil {
		return err
	}

	err = sender.sendStopMessages(stopMessages)
	if err != nil {
		return err
	}

	return nil
}

func (sender *Sender) sendStartMessages(startMessages []models.QueueStartMessage) error {
	startMessagesToSave := []models.QueueStartMessage{}
	startMessagesToDelete := []models.QueueStartMessage{}

	for _, startMessage := range startMessages {
		if startMessage.IsExpired(sender.timeProvider.Time()) {
			startMessagesToDelete = append(startMessagesToDelete, startMessage)
		} else if startMessage.IsTimeToSend(sender.timeProvider.Time()) {
			if sender.verifyStartMessageShouldBeSent(startMessage) {
				messageToSend := models.StartMessage{
					AppGuid:        startMessage.AppGuid,
					AppVersion:     startMessage.AppVersion,
					InstanceIndex:  startMessage.IndexToStart,
					RunningIndices: sender.runningIndicesForApp(startMessage.AppGuid, startMessage.AppVersion),
				}
				err := sender.messageBus.Publish("hm9000.start", messageToSend.ToJSON())
				if err != nil {
					return err
				}
				if startMessage.KeepAlive == 0 {
					startMessagesToDelete = append(startMessagesToDelete, startMessage)
				} else {
					startMessage.SentOn = sender.timeProvider.Time().Unix()
					startMessagesToSave = append(startMessagesToSave, startMessage)
				}
			} else {
				startMessagesToDelete = append(startMessagesToDelete, startMessage)
			}
		}
	}

	err := sender.store.SaveQueueStartMessages(startMessagesToSave)
	if err != nil {
		return err
	}
	err = sender.store.DeleteQueueStartMessages(startMessagesToDelete)
	if err != nil {
		return err
	}

	return nil
}

func (sender *Sender) sendStopMessages(stopMessages []models.QueueStopMessage) error {
	stopMessagesToSave := []models.QueueStopMessage{}
	stopMessagesToDelete := []models.QueueStopMessage{}

	for _, stopMessage := range stopMessages {
		if stopMessage.IsExpired(sender.timeProvider.Time()) {
			stopMessagesToDelete = append(stopMessagesToDelete, stopMessage)
		} else if stopMessage.IsTimeToSend(sender.timeProvider.Time()) {
			if sender.verifyStopMessageShouldBeSent(stopMessage) {
				actual := sender.storecache.RunningByInstance[stopMessage.InstanceGuid]
				messageToSend := models.StopMessage{
					AppGuid:        actual.AppGuid,
					AppVersion:     actual.AppVersion,
					InstanceIndex:  actual.InstanceIndex,
					InstanceGuid:   stopMessage.InstanceGuid,
					RunningIndices: sender.runningIndicesForApp(actual.AppGuid, actual.AppVersion),
				}
				err := sender.messageBus.Publish("hm9000.stop", messageToSend.ToJSON())
				if err != nil {
					return err
				}
				if stopMessage.KeepAlive == 0 {
					stopMessagesToDelete = append(stopMessagesToDelete, stopMessage)
				} else {
					stopMessage.SentOn = sender.timeProvider.Time().Unix()
					stopMessagesToSave = append(stopMessagesToSave, stopMessage)
				}
			} else {
				stopMessagesToDelete = append(stopMessagesToDelete, stopMessage)
			}
		}
	}

	err := sender.store.SaveQueueStopMessages(stopMessagesToSave)
	if err != nil {
		return err
	}
	err = sender.store.DeleteQueueStopMessages(stopMessagesToDelete)
	if err != nil {
		return err
	}

	return nil
}

func (sender *Sender) verifyStartMessageShouldBeSent(message models.QueueStartMessage) bool {
	appKey := sender.storecache.Key(message.AppGuid, message.AppVersion)
	desired, ok := sender.storecache.DesiredByApp[appKey]
	if !ok {
		//app is no longer desired, don't start the instance
		return false
	}
	if desired.NumberOfInstances <= message.IndexToStart {
		//instance index is beyond the desired # of instances, don't start the instance
		return false
	}
	allRunningInstances, ok := sender.storecache.RunningByApp[appKey]
	if !ok {
		//there are no running instances, start the instance
		return true
	}
	for _, heartbeat := range allRunningInstances {
		if heartbeat.InstanceIndex == message.IndexToStart {
			//there is already an instance running at that index, don't start another
			return false
		}
	}

	//there was no instance running at that index, start the instance
	return true
}

func (sender *Sender) verifyStopMessageShouldBeSent(message models.QueueStopMessage) bool {
	instanceToStop, ok := sender.storecache.RunningByInstance[message.InstanceGuid]
	if !ok {
		//there was no running instance found with that guid, don't send a stop message
		return false
	}
	appKey := sender.storecache.Key(instanceToStop.AppGuid, instanceToStop.AppVersion)
	desired, ok := sender.storecache.DesiredByApp[appKey]
	if !ok {
		//there is no desired app for this instance, send the stop message
		return true
	}
	if desired.NumberOfInstances <= instanceToStop.InstanceIndex {
		//the instance index is beyond the desired # of instances, stop the app
		return true
	}
	allRunningInstances, _ := sender.storecache.RunningByApp[appKey]
	for _, heartbeat := range allRunningInstances {
		if heartbeat.InstanceIndex == instanceToStop.InstanceIndex && heartbeat.InstanceGuid != instanceToStop.InstanceGuid {
			// there is *another* instance reporting at this index,
			// so the instance-to-stop is an extra instance reporting on a desired index, stop it
			return true
		}
	}

	//the instance index is within the desired # of instances
	//there are no other instances running on this index
	//don't stop the instance
	return false
}

func (sender *Sender) runningIndicesForApp(appGuid string, appVersion string) models.RunningIndices {
	runningInstances := sender.storecache.RunningByApp[sender.storecache.Key(appGuid, appVersion)]
	runningIndices := models.RunningIndices{}

	for _, instance := range runningInstances {
		runningIndices.IncrementIndex(instance.InstanceIndex)
	}

	return runningIndices
}
