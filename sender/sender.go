package sender

import (
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
)

type Sender struct {
	store        store.Store
	messageBus   cfmessagebus.MessageBus
	timeProvider timeprovider.TimeProvider

	actualStates      []models.InstanceHeartbeat
	runningByApp      map[string][]models.InstanceHeartbeat
	runningByInstance map[string]models.InstanceHeartbeat
}

func New(store store.Store, messageBus cfmessagebus.MessageBus, timeProvider timeprovider.TimeProvider) *Sender {
	return &Sender{
		store:        store,
		messageBus:   messageBus,
		timeProvider: timeProvider,
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
	err = sender.fetchStateAndGenerateLookupTables()
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
			actual := sender.runningByInstance[stopMessage.InstanceGuid]
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

func (sender *Sender) runningIndicesForApp(appGuid string, appVersion string) models.RunningIndices {
	runningInstances := sender.runningByApp[appGuid+"-"+appVersion]
	runningIndices := models.RunningIndices{}

	for _, instance := range runningInstances {
		runningIndices.IncrementIndex(instance.InstanceIndex)
	}

	return runningIndices
}

func (sender *Sender) fetchStateAndGenerateLookupTables() (err error) {
	sender.actualStates, err = sender.store.GetActualState()
	if err != nil {
		return
	}
	sender.runningByApp = make(map[string][]models.InstanceHeartbeat, 0)
	sender.runningByInstance = make(map[string]models.InstanceHeartbeat, 0)

	for _, actual := range sender.actualStates {
		sender.runningByInstance[actual.InstanceGuid] = actual

		key := actual.AppGuid + "-" + actual.AppVersion
		value, ok := sender.runningByApp[key]
		if ok {
			sender.runningByApp[key] = append(value, actual)
		} else {
			sender.runningByApp[key] = []models.InstanceHeartbeat{actual}
		}
	}
	return nil
}
