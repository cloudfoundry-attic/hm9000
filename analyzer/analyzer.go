package analyzer

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
)

type Analyzer struct {
	store      store.Store
	storecache *storecache.StoreCache

	logger       logger.Logger
	timeProvider timeprovider.TimeProvider
	conf         config.Config
}

func New(store store.Store, timeProvider timeprovider.TimeProvider, logger logger.Logger, conf config.Config) *Analyzer {
	return &Analyzer{
		store:        store,
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

	allStartMessages := []models.PendingStartMessage{}
	allStopMessages := []models.PendingStopMessage{}
	allCrashCounts := []models.CrashCount{}

	for _, app := range analyzer.storecache.Apps {
		startMessages, stopMessages, crashCounts := newAppAnalyzer(app, analyzer.timeProvider.Time(), analyzer.storecache, analyzer.logger, analyzer.conf).analyzeApp()
		allStartMessages = append(allStartMessages, startMessages...)
		allStopMessages = append(allStopMessages, stopMessages...)
		allCrashCounts = append(allCrashCounts, crashCounts...)
	}

	err = analyzer.store.SaveCrashCounts(allCrashCounts...)

	if err != nil {
		analyzer.logger.Error("Analyzer failed to save crash counts", err)
		return err
	}

	err = analyzer.store.SavePendingStartMessages(allStartMessages...)

	if err != nil {
		analyzer.logger.Error("Analyzer failed to enqueue start messages", err)
		return err
	}

	err = analyzer.store.SavePendingStopMessages(allStopMessages...)
	if err != nil {
		analyzer.logger.Error("Analyzer failed to enqueue stop messages", err)
		return err
	}

	return nil
}
