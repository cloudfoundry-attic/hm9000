package storecassandra

import (
	"github.com/cloudfoundry/hm9000/models"
	"tux21b.org/v1/gocql"
)

func (s *StoreCassandra) SaveHeartbeat(heartbeat models.Heartbeat) error {
	batch := s.newBatch()

	for _, state := range heartbeat.InstanceHeartbeats {
		batch.Query(`INSERT INTO ActualStates (app_guid, app_version, instance_guid, instance_index, state, state_timestamp, cc_partition, expires) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			state.AppGuid,
			state.AppVersion,
			state.InstanceGuid,
			int32(state.InstanceIndex),
			state.State,
			int64(state.StateTimestamp),
			state.CCPartition,
			s.timeProvider.Time().Unix()+int64(s.conf.HeartbeatTTL()))
	}

	return s.session.ExecuteBatch(batch)
}

func (s *StoreCassandra) GetActualState() (map[string]models.InstanceHeartbeat, error) {
	result := map[string]models.InstanceHeartbeat{}
	actualStates, err := s.getActualState("", "")

	if err != nil {
		return result, err
	}

	for _, actualState := range actualStates {
		result[actualState.StoreKey()] = actualState
	}

	return result, err
}

func (s *StoreCassandra) getActualState(optionalAppGuid string, optionalAppVersion string) ([]models.InstanceHeartbeat, error) {
	result := []models.InstanceHeartbeat{}
	var err error
	var iter *gocql.Iter

	if optionalAppGuid == "" {
		iter = s.session.Query(`SELECT app_guid, app_version, instance_guid, instance_index, state, state_timestamp, cc_partition, expires FROM ActualStates`).Iter()
	} else {
		iter = s.session.Query(`SELECT app_guid, app_version, instance_guid, instance_index, state, state_timestamp, cc_partition, expires FROM ActualStates WHERE app_guid = ? AND app_version = ?`, optionalAppGuid, optionalAppVersion).Iter()
	}

	var appGuid, appVersion, instanceGuid, state, ccPartition string
	var instanceIndex int32
	var stateTimestamp, expires int64

	currentTime := s.timeProvider.Time().Unix()

	batch := s.newBatch()

	for iter.Scan(&appGuid, &appVersion, &instanceGuid, &instanceIndex, &state, &stateTimestamp, &ccPartition, &expires) {
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
