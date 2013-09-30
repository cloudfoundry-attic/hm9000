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
		return false
	}
	if desired.NumberOfInstances <= message.IndexToStart {
		return false
	}
	actual, ok := sender.storecache.RunningByApp[appKey]
	if !ok {
		return true
	}
	for _, heartbeat := range actual {
		if heartbeat.InstanceIndex == message.IndexToStart {
			return false
		}
	}
	return true
}

func (sender *Sender) runningIndicesForApp(appGuid string, appVersion string) models.RunningIndices {
	runningInstances := sender.storecache.RunningByApp[sender.storecache.Key(appGuid, appVersion)]
	runningIndices := models.RunningIndices{}

	for _, instance := range runningInstances {
		runningIndices.IncrementIndex(instance.InstanceIndex)
	}

	return runningIndices
}
