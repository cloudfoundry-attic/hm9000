package sender

import (
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"sort"
)

type Sender struct {
	store      store.Store
	conf       config.Config
	storecache *storecache.StoreCache
	logger     logger.Logger

	messageBus   cfmessagebus.MessageBus
	timeProvider timeprovider.TimeProvider
}

func New(store store.Store, conf config.Config, messageBus cfmessagebus.MessageBus, timeProvider timeprovider.TimeProvider, logger logger.Logger) *Sender {
	return &Sender{
		store:        store,
		conf:         conf,
		logger:       logger,
		messageBus:   messageBus,
		timeProvider: timeProvider,
		storecache:   storecache.New(store),
	}
}

func (sender *Sender) Send() error {
	startMessages, err := sender.store.GetPendingStartMessages()
	if err != nil {
		sender.logger.Error("Failed to fetch start messages", err)
		return err
	}

	stopMessages, err := sender.store.GetPendingStopMessages()
	if err != nil {
		sender.logger.Error("Failed to fetch stop messages", err)
		return err
	}

	err = sender.storecache.Load(sender.timeProvider.Time())
	if err != nil {
		sender.logger.Error("Failed to load desired and actual states", err)
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

type SortablePendingStartMessages []models.PendingStartMessage

func (s SortablePendingStartMessages) Len() int           { return len(s) }
func (s SortablePendingStartMessages) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s SortablePendingStartMessages) Less(i, j int) bool { return s[i].Priority < s[j].Priority }

func (sender *Sender) sendStartMessages(startMessages []models.PendingStartMessage) error {
	startMessagesToSave := []models.PendingStartMessage{}
	startMessagesToDelete := []models.PendingStartMessage{}

	sortedStartMessages := make(SortablePendingStartMessages, len(startMessages))
	for i, message := range startMessages {
		sortedStartMessages[i] = message
	}
	sort.Sort(sort.Reverse(sortedStartMessages))

	numSent := 0
	maxSent := sender.conf.SenderMessageLimit

	for _, startMessage := range sortedStartMessages {
		if startMessage.IsExpired(sender.timeProvider.Time()) {
			sender.logger.Info("Deleting expired start message", startMessage.LogDescription())
			startMessagesToDelete = append(startMessagesToDelete, startMessage)
		} else if startMessage.IsTimeToSend(sender.timeProvider.Time()) {
			if sender.verifyStartMessageShouldBeSent(startMessage) {
				if numSent < maxSent {
					messageToSend := models.StartMessage{
						AppGuid:       startMessage.AppGuid,
						AppVersion:    startMessage.AppVersion,
						InstanceIndex: startMessage.IndexToStart,
						MessageId:     startMessage.MessageId,
					}
					sender.logger.Info("Sending message", startMessage.LogDescription())
					err := sender.messageBus.Publish(sender.conf.SenderNatsStartSubject, messageToSend.ToJSON())
					if err != nil {
						sender.logger.Error("Failed to send start message", err, startMessage.LogDescription())
						return err
					}
					if startMessage.KeepAlive == 0 {
						sender.logger.Info("Deleting sent start message with no keep alive", startMessage.LogDescription())
						startMessagesToDelete = append(startMessagesToDelete, startMessage)
					} else {
						startMessage.SentOn = sender.timeProvider.Time().Unix()
						startMessagesToSave = append(startMessagesToSave, startMessage)
					}
					numSent += 1
				}
			} else {
				sender.logger.Info("Deleting start message that will not be sent", startMessage.LogDescription())
				startMessagesToDelete = append(startMessagesToDelete, startMessage)
			}
		} else {
			sender.logger.Info("Skipping start message whose time has not come", startMessage.LogDescription(), map[string]string{
				"current time": sender.timeProvider.Time().String(),
			})
		}
	}

	err := sender.store.SavePendingStartMessages(startMessagesToSave)
	if err != nil {
		sender.logger.Error("Failed to save start messages to send", err)
		return err
	}
	err = sender.store.DeletePendingStartMessages(startMessagesToDelete)
	if err != nil {
		sender.logger.Error("Failed to delete start messages", err)
		return err
	}

	return nil
}

func (sender *Sender) sendStopMessages(stopMessages []models.PendingStopMessage) error {
	stopMessagesToSave := []models.PendingStopMessage{}
	stopMessagesToDelete := []models.PendingStopMessage{}

	for _, stopMessage := range stopMessages {
		if stopMessage.IsExpired(sender.timeProvider.Time()) {
			sender.logger.Info("Deleting expired stop message", stopMessage.LogDescription())
			stopMessagesToDelete = append(stopMessagesToDelete, stopMessage)
		} else if stopMessage.IsTimeToSend(sender.timeProvider.Time()) {
			shouldSend, isDuplicate := sender.verifyStopMessageShouldBeSent(stopMessage)
			if shouldSend {
				actual := sender.storecache.HeartbeatingInstancesByGuid[stopMessage.InstanceGuid]
				messageToSend := models.StopMessage{
					AppGuid:       actual.AppGuid,
					AppVersion:    actual.AppVersion,
					InstanceIndex: actual.InstanceIndex,
					InstanceGuid:  stopMessage.InstanceGuid,
					IsDuplicate:   isDuplicate,
					MessageId:     stopMessage.MessageId,
				}
				err := sender.messageBus.Publish(sender.conf.SenderNatsStopSubject, messageToSend.ToJSON())
				if err != nil {
					sender.logger.Error("Failed to send stop message", err, stopMessage.LogDescription())
					return err
				}
				if stopMessage.KeepAlive == 0 {
					sender.logger.Info("Deleting sent stop message with no keep alive", stopMessage.LogDescription())
					stopMessagesToDelete = append(stopMessagesToDelete, stopMessage)
				} else {
					stopMessage.SentOn = sender.timeProvider.Time().Unix()
					stopMessagesToSave = append(stopMessagesToSave, stopMessage)
				}
			} else {
				sender.logger.Info("Deleting stop message that will not be sent", stopMessage.LogDescription())
				stopMessagesToDelete = append(stopMessagesToDelete, stopMessage)
			}
		} else {
			sender.logger.Info("Skipping stop message whose time has not come", stopMessage.LogDescription())
		}
	}

	err := sender.store.SavePendingStopMessages(stopMessagesToSave)
	if err != nil {
		sender.logger.Error("Failed to save stop messages to send", err)
		return err
	}
	err = sender.store.DeletePendingStopMessages(stopMessagesToDelete)
	if err != nil {
		sender.logger.Error("Failed to delete stop messages", err)
		return err
	}

	return nil
}

func (sender *Sender) verifyStartMessageShouldBeSent(message models.PendingStartMessage) bool {
	appKey := sender.storecache.Key(message.AppGuid, message.AppVersion)
	desired, hasDesiredState := sender.storecache.DesiredByApp[appKey]
	if !hasDesiredState {
		//app is no longer desired, don't start the instance
		sender.logger.Info("Skipping sending start message: app is no longer desired", message.LogDescription())
		return false
	}
	if desired.NumberOfInstances <= message.IndexToStart {
		//instance index is beyond the desired # of instances, don't start the instance
		sender.logger.Info("Skipping sending start message: instance index is beyond the desired # of instances",
			message.LogDescription(), desired.LogDescription())
		return false
	}
	allHeartbeatingInstances, hasHeartbeatingInstances := sender.storecache.HeartbeatingInstancesByApp[appKey]
	if !hasHeartbeatingInstances {
		//there are no running instances, start the instance
		sender.logger.Info("Sending start message: instance is desired but not running",
			message.LogDescription(), desired.LogDescription())
		return true
	}

	for _, heartbeat := range allHeartbeatingInstances {
		if heartbeat.InstanceIndex == message.IndexToStart && heartbeat.State != models.InstanceStateCrashed {
			//there is already an instance running at that index, don't start another
			sender.logger.Info("Skipping sending start message: instance is already running",
				message.LogDescription(), desired.LogDescription(), heartbeat.LogDescription())
			return false
		}
	}

	//there was no instance running at that index, start the instance
	sender.logger.Info("Sending start message: instance is not running at desired index",
		message.LogDescription(), desired.LogDescription())
	return true
}

func (sender *Sender) verifyStopMessageShouldBeSent(message models.PendingStopMessage) (bool, isDuplicate bool) {
	instanceToStop, found := sender.storecache.HeartbeatingInstancesByGuid[message.InstanceGuid]
	if !found {
		//there was no running instance found with that guid, don't send a stop message
		sender.logger.Info("Skipping sending stop message: instance is no longer running", message.LogDescription())
		return false, false
	}
	appKey := sender.storecache.Key(instanceToStop.AppGuid, instanceToStop.AppVersion)
	desired, found := sender.storecache.DesiredByApp[appKey]
	if !found {
		//there is no desired app for this instance, send the stop message
		sender.logger.Info("Sending stop message: instance is running, app is no longer desired",
			message.LogDescription(),
			instanceToStop.LogDescription())
		return true, false
	}
	if desired.NumberOfInstances <= instanceToStop.InstanceIndex {
		//the instance index is beyond the desired # of instances, stop the app
		sender.logger.Info("Sending stop message: index of instance to stop is beyond desired # of instances",
			message.LogDescription(),
			instanceToStop.LogDescription(),
			desired.LogDescription())
		return true, false
	}
	allRunningInstances, _ := sender.storecache.HeartbeatingInstancesByApp[appKey]
	for _, heartbeat := range allRunningInstances {
		if heartbeat.InstanceIndex == instanceToStop.InstanceIndex &&
			heartbeat.InstanceGuid != instanceToStop.InstanceGuid &&
			heartbeat.State != models.InstanceStateCrashed {
			// there is *another* instance reporting at this index,
			// so the instance-to-stop is an extra instance reporting on a desired index, stop it
			sender.logger.Info("Sending stop message: instance is a duplicate running at a desired index",
				message.LogDescription(),
				instanceToStop.LogDescription(),
				desired.LogDescription())
			return true, true
		}
	}

	//the instance index is within the desired # of instances
	//there are no other instances running on this index
	//don't stop the instance
	sender.logger.Info("Skipping sending stop message: instance is running on a desired index (and there are no other instances running at that index)",
		message.LogDescription(),
		instanceToStop.LogDescription(),
		desired.LogDescription())
	return false, false
}
