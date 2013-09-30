package analyzer

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/outbox"
	"github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
)

type Analyzer struct {
	store      store.Store
	storecache *storecache.StoreCache

	logger       logger.Logger
	outbox       outbox.Outbox
	timeProvider timeprovider.TimeProvider
	conf         config.Config
}

func New(store store.Store, outbox outbox.Outbox, timeProvider timeprovider.TimeProvider, logger logger.Logger, conf config.Config) *Analyzer {
	return &Analyzer{
		store:        store,
		outbox:       outbox,
		timeProvider: timeProvider,
		logger:       logger,
		conf:         conf,
		storecache:   storecache.New(store),
	}
}

func (analyzer *Analyzer) Analyze() error {
	err := analyzer.storecache.Load(analyzer.timeProvider.Time())
	if err != nil {
		analyzer.logger.Error("Failed to load desired and actual states", err)
		return err
	}

	allStartMessages := []models.QueueStartMessage{}
	allStopMessages := []models.QueueStopMessage{}

	for appVersionKey := range analyzer.storecache.SetOfApps {
		desired := analyzer.storecache.DesiredByApp[appVersionKey]
		runningInstances := analyzer.storecache.RunningByApp[appVersionKey]
		startMessages, stopMessages := analyzer.analyzeApp(desired, runningInstances)
		allStartMessages = append(allStartMessages, startMessages...)
		allStopMessages = append(allStopMessages, stopMessages...)
	}

	err = analyzer.outbox.Enqueue(allStartMessages, allStopMessages)
	if err != nil {
		analyzer.logger.Error("Analyzer failed to enqueue messages", err)
		return err
	}
	return nil
}
