package metricsserver

import (
	"github.com/cloudfoundry/go_cfmessagebus"
	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/instrumentation"
	"github.com/cloudfoundry/loggregatorlib/cfcomponent/registrars/collectorregistrar"
)

type MetricsServer struct {
	storecache   *storecache.StoreCache
	steno        *gosteno.Logger
	timeProvider timeprovider.TimeProvider
	config       config.Config
	messageBus   cfmessagebus.MessageBus
}

func New(messageBus cfmessagebus.MessageBus, steno *gosteno.Logger, store store.Store, timeProvider timeprovider.TimeProvider, conf config.Config) *MetricsServer {
	storecache := storecache.New(store)
	return &MetricsServer{
		messageBus:   messageBus,
		storecache:   storecache,
		timeProvider: timeProvider,
		steno:        steno,
		config:       conf,
	}
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

	for _, app := range s.storecache.Apps {
		numberOfMissingIndicesForApp := app.NumberOfDesiredInstances() - app.NumberOfDesiredIndicesReporting()
		if app.IsDesired() {
			if numberOfMissingIndicesForApp == 0 {
				NumberOfAppsWithAllInstancesReporting++
			} else {
				NumberOfAppsWithMissingInstances++
			}
		} else {
			if app.HasStartingOrRunningInstances() {
				NumberOfUndesiredRunningApps++
			}
		}

		NumberOfRunningInstances += app.NumberOfStartingOrRunningInstances()
		NumberOfMissingIndices += numberOfMissingIndicesForApp
		NumberOfCrashedInstances += app.NumberOfCrashedInstances()
		NumberOfCrashedIndices += app.NumberOfCrashedIndices()
	}

	return
}

func (s *MetricsServer) Ok() bool {
	return true
}

func (s *MetricsServer) Start() error {
	component, err := cfcomponent.NewComponent(
		s.steno,
		"HM9000",
		0,
		s,
		uint32(s.config.MetricsServerPort),
		[]string{s.config.MetricsServerUser, s.config.MetricsServerPassword},
		[]instrumentation.Instrumentable{s},
	)

	if err != nil {
		return err
	}

	go component.StartMonitoringEndpoints()

	err = collectorregistrar.NewCollectorRegistrar(s.messageBus, s.steno).RegisterWithCollector(component)

	return err
}
