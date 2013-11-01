package storecassandra

import (
	"github.com/cloudfoundry/hm9000/models"
	"time"
	"tux21b.org/v1/gocql"
)

func (s *StoreCassandra) SaveDesiredState(desiredStates ...models.DesiredAppState) error {
	batch := s.newBatch()

	for _, state := range desiredStates {
		batch.Query(`INSERT INTO DesiredStates (app_guid, app_version, number_of_instances, memory, state, package_state, updated_at, expires) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, state.AppGuid, state.AppVersion, state.NumberOfInstances, state.Memory, state.State, state.PackageState, int64(state.UpdatedAt.Unix()), s.timeProvider.Time().Unix()+int64(s.conf.DesiredStateTTL()))
	}

	return s.session.ExecuteBatch(batch)
}

func (s *StoreCassandra) GetDesiredState() (map[string]models.DesiredAppState, error) {
	result := map[string]models.DesiredAppState{}
	desiredStates, err := s.getDesiredState("", "")

	if err != nil {
		return result, err
	}

	for _, desiredState := range desiredStates {
		result[desiredState.StoreKey()] = desiredState
	}

	return result, err
}

func (s *StoreCassandra) getDesiredState(optionalAppGuid string, optionalAppVersion string) ([]models.DesiredAppState, error) {
	result := []models.DesiredAppState{}
	var err error
	var iter *gocql.Iter

	if optionalAppGuid == "" {
		iter = s.session.Query(`SELECT app_guid, app_version, number_of_instances, memory, state, package_state, updated_at, expires FROM DesiredStates`).Iter()
	} else {
		iter = s.session.Query(`SELECT app_guid, app_version, number_of_instances, memory, state, package_state, updated_at, expires FROM DesiredStates WHERE app_guid = ? AND app_version = ?`, optionalAppGuid, optionalAppVersion).Iter()
	}

	var appGuid, appVersion, state, packageState string
	var numberOfInstances, memory int32
	var updatedAt, expires int64

	currentTime := s.timeProvider.Time().Unix()

	batch := s.newBatch()

	for iter.Scan(&appGuid, &appVersion, &numberOfInstances, &memory, &state, &packageState, &updatedAt, &expires) {
		if expires <= currentTime {
			s.deleteDesiredState(appGuid, appVersion, batch)
		} else {
			desiredState := models.DesiredAppState{
				AppGuid:           appGuid,
				AppVersion:        appVersion,
				NumberOfInstances: int(numberOfInstances),
				Memory:            int(memory),
				State:             models.AppState(state),
				PackageState:      models.AppPackageState(packageState),
				UpdatedAt:         time.Unix(updatedAt, 0),
			}
			result = append(result, desiredState)
		}
	}

	err = iter.Close()

	if err != nil {
		return result, err
	}

	err = s.session.ExecuteBatch(batch)
	return result, err
}

func (s *StoreCassandra) deleteDesiredState(appGuid string, appVersion string, batch *gocql.Batch) {
	batch.Query(`DELETE FROM DesiredStates WHERE app_guid=? AND app_version=?`, appGuid, appVersion)
}

func (s *StoreCassandra) DeleteDesiredState(desiredStates ...models.DesiredAppState) error {
	batch := s.newBatch()

	for _, state := range desiredStates {
		s.deleteDesiredState(state.AppGuid, state.AppVersion, batch)
	}

	return s.session.ExecuteBatch(batch)
}
