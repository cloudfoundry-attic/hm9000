package storecassandra

import (
	"github.com/cloudfoundry/hm9000/models"
	"tux21b.org/v1/gocql"
)

func (s *StoreCassandra) SaveHeartbeat(heartbeat models.Heartbeat) error {
	summary := heartbeat.HeartbeatSummary()
	iter := s.session.Query(`SELECT app_guid, app_version, instance_guid FROM ActualStates WHERE dea_guid = ?`, heartbeat.DeaGuid).Iter()

	batch := s.newBatch()

	var appGuid, appVersion, instanceGuid string
	for iter.Scan(&appGuid, &appVersion, &instanceGuid) {
		_, exists := summary.InstanceHeartbeatSummaries[instanceGuid]
		if !exists {
			batch.Query(`DELETE FROM ActualStates WHERE app_guid = ? AND app_version = ? AND instance_guid = ?`, appGuid, appVersion, instanceGuid)
		}
	}

	err := iter.Close()
	if err != nil {
		return err
	}

	for _, state := range heartbeat.InstanceHeartbeats {
		batch.Query(`INSERT INTO ActualStates (app_guid, app_version, instance_guid, instance_index, state, state_timestamp, cc_partition, dea_guid, expires) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			state.AppGuid,
			state.AppVersion,
			state.InstanceGuid,
			int32(state.InstanceIndex),
			state.State,
			int64(state.StateTimestamp),
			state.CCPartition,
			state.DeaGuid,
			s.timeProvider.Time().Unix()+int64(s.conf.HeartbeatTTL()))
	}

	return s.session.ExecuteBatch(batch)
}

func (s *StoreCassandra) GetInstanceHeartbeats() ([]models.InstanceHeartbeat, error) {
	return s.getActualState("", "")
}

func (s *StoreCassandra) GetInstanceHeartbeatsForApp(appGuid string, appVersion string) ([]models.InstanceHeartbeat, error) {
	return s.getActualState(appGuid, appVersion)
}

func (s *StoreCassandra) getActualState(optionalAppGuid string, optionalAppVersion string) ([]models.InstanceHeartbeat, error) {
	result := []models.InstanceHeartbeat{}
	var err error
	var iter *gocql.Iter

	if optionalAppGuid == "" {
		iter = s.session.Query(`SELECT app_guid, app_version, instance_guid, instance_index, state, state_timestamp, cc_partition, dea_guid, expires FROM ActualStates`).Iter()
	} else {
		iter = s.session.Query(`SELECT app_guid, app_version, instance_guid, instance_index, state, state_timestamp, cc_partition, dea_guid, expires FROM ActualStates WHERE app_guid = ? AND app_version = ?`, optionalAppGuid, optionalAppVersion).Iter()
	}

	var appGuid, appVersion, instanceGuid, state, ccPartition, deaGuid string
	var instanceIndex int32
	var stateTimestamp, expires int64

	currentTime := s.timeProvider.Time().Unix()

	batch := s.newBatch()

	for iter.Scan(&appGuid, &appVersion, &instanceGuid, &instanceIndex, &state, &stateTimestamp, &ccPartition, &deaGuid, &expires) {
		if expires <= currentTime {
			batch.Query(`DELETE FROM ActualStates WHERE app_guid=? AND app_version=? AND instance_guid = ?`, appGuid, appVersion, instanceGuid)
		} else {
			actualState := models.InstanceHeartbeat{
				CCPartition:    ccPartition,
				AppGuid:        appGuid,
				AppVersion:     appVersion,
				InstanceGuid:   instanceGuid,
				InstanceIndex:  int(instanceIndex),
				State:          models.InstanceState(state),
				StateTimestamp: float64(stateTimestamp),
				DeaGuid:        deaGuid,
			}
			result = append(result, actualState)
		}
	}

	err = iter.Close()

	if err != nil {
		return result, err
	}

	err = s.session.ExecuteBatch(batch)

	return result, err
}

func (s *StoreCassandra) TruncateActualState() error {
	//this is for the performance tests, only.
	return s.session.Query(`TRUNCATE ActualStates`).Exec()
}
