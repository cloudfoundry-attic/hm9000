package metricsserver

import (
	"github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
)

type MetricsServer struct {
	storecache   *storecache.StoreCache
	timeProvider timeprovider.TimeProvider
}

func New(store store.Store, timeProvider timeprovider.TimeProvider) *MetricsServer {
	storecache := storecache.New(store)
	return &MetricsServer{storecache: storecache, timeProvider: timeProvider}
}

func (s *MetricsServer) Emit() (context instrumentation.Context) {
	context.Name = "HM9000"

	NumberOfAppsWithAllInstancesReporting := 0
	NumberOfAppsWithMissingInstances := 0
	NumberOfUndesiredRunningApps := 0
	NumberOfRunningInstances := 0
	NumberOfMissingIndices := 0
	NumberOfCrashedInstances := 0
	NumberOfCrashedIndices := 0

	defer func() {
		context.Metrics = append(context.Metrics, instrumentation.Metric{
			Name:  "NumberOfAppsWithAllInstancesReporting",
			Value: NumberOfAppsWithAllInstancesReporting,
		})

		context.Metrics = append(context.Metrics, instrumentation.Metric{
			Name:  "NumberOfAppsWithMissingInstances",
			Value: NumberOfAppsWithMissingInstances,
		})

		context.Metrics = append(context.Metrics, instrumentation.Metric{
			Name:  "NumberOfUndesiredRunningApps",
			Value: NumberOfUndesiredRunningApps,
		})

		context.Metrics = append(context.Metrics, instrumentation.Metric{
			Name:  "NumberOfRunningInstances",
			Value: NumberOfRunningInstances,
		})

		context.Metrics = append(context.Metrics, instrumentation.Metric{
			Name:  "NumberOfMissingIndices",
			Value: NumberOfMissingIndices,
		})

		context.Metrics = append(context.Metrics, instrumentation.Metric{
			Name:  "NumberOfCrashedInstances",
			Value: NumberOfCrashedInstances,
		})

		context.Metrics = append(context.Metrics, instrumentation.Metric{
			Name:  "NumberOfCrashedIndices",
			Value: NumberOfCrashedIndices,
		})
	}()

	err := s.storecache.Load(s.timeProvider.Time())
	if err != nil {
		NumberOfAppsWithAllInstancesReporting = -1
		NumberOfAppsWithMissingInstances = -1
		NumberOfUndesiredRunningApps = -1
		NumberOfRunningInstances = -1
		NumberOfMissingIndices = -1
		NumberOfCrashedInstances = -1
		NumberOfCrashedIndices = -1
		return
	}

	for key, _ := range s.storecache.SetOfApps {
		appMetrics := NewAppMetrics(s.storecache.DesiredByApp[key], s.storecache.HeartbeatingInstancesByApp[key])
		if appMetrics.HasAllInstancesReporting {
			NumberOfAppsWithAllInstancesReporting++
		}
		if appMetrics.HasMissingInstances {
			NumberOfAppsWithMissingInstances++
		}
		if appMetrics.IsRunningButUndesired {
			NumberOfUndesiredRunningApps++
		}
		NumberOfRunningInstances += appMetrics.NumberOfRunningInstances
		NumberOfMissingIndices += appMetrics.NumberOfMissingIndices
		NumberOfCrashedInstances += appMetrics.NumberOfCrashedInstances
		NumberOfCrashedIndices += appMetrics.NumberOfCrashedIndices
	}

	return
}
