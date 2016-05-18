package analyzer

import (
	"errors"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"
)

type Analyzer struct {
	store store.Store

	logger lager.Logger
	clock  clock.Clock
	conf   *config.Config
}

func New(store store.Store, clock clock.Clock, logger lager.Logger, conf *config.Config) *Analyzer {
	return &Analyzer{
		store:  store,
		clock:  clock,
		logger: logger,
		conf:   conf,
	}
}

func (analyzer *Analyzer) Analyze(appQueue *models.AppQueue) (map[string]*models.App, error) {
	defer close(appQueue.DoneAnalyzing)

	err := analyzer.store.VerifyFreshness(analyzer.clock.Now())
	if err != nil {
		analyzer.logger.Error("Store is not fresh", err)
		return nil, err
	}

	actualApps, err := analyzer.store.GetApps()

	appsPendingRemoval := make(map[string]*models.App)
	for k, v := range actualApps {
		appsPendingRemoval[k] = v
	}

	if err != nil {
		analyzer.logger.Error("Failed to fetch apps", err)
		return nil, err
	}

	existingPendingStartMessages, err := analyzer.store.GetPendingStartMessages()
	if err != nil {
		analyzer.logger.Error("Failed to fetch pending start messages", err)
		return nil, err
	}

	existingPendingStopMessages, err := analyzer.store.GetPendingStopMessages()
	if err != nil {
		analyzer.logger.Error("Failed to fetch pending stop messages", err)
		return nil, err
	}

	allStartMessages := []models.PendingStartMessage{}
	allStopMessages := []models.PendingStopMessage{}
	allCrashCounts := []models.CrashCount{}

	for desiredAppBatch := range appQueue.DesiredApps {
		for _, desiredApp := range desiredAppBatch {
			app := actualApps[desiredApp.StoreKey()]

			if app == nil {
				app = models.NewApp(desiredApp.AppGuid, desiredApp.AppVersion, desiredApp, nil, nil)
				actualApps[desiredApp.StoreKey()] = app
			} else {
				// found  a valid app. remove it from the list of apps that might need to be removed
				app.Desired = desiredApp
				delete(appsPendingRemoval, desiredApp.StoreKey())
			}

			startMessages, stopMessages, crashCounts := newAppAnalyzer(app, analyzer.clock.Now(), existingPendingStartMessages, existingPendingStopMessages, analyzer.logger, analyzer.conf).analyzeApp()
			for _, startMessage := range startMessages {
				allStartMessages = append(allStartMessages, startMessage)
			}
			for _, stopMessage := range stopMessages {
				allStopMessages = append(allStopMessages, stopMessage)
			}
			allCrashCounts = append(allCrashCounts, crashCounts...)
		}
	}

	if !appQueue.FetchDesiredAppsSuccess() {
		err := errors.New("Desired State Fetcher exited unsuccessfully.")
		analyzer.logger.Error("Analyzer is stopping", err)
		return nil, err
	}

	for _, app := range appsPendingRemoval {
		_, stopMessages, crashCounts := newAppAnalyzer(app, analyzer.clock.Now(), existingPendingStartMessages, existingPendingStopMessages, analyzer.logger, analyzer.conf).analyzeApp()
		for _, stopMessage := range stopMessages {
			allStopMessages = append(allStopMessages, stopMessage)
		}
		allCrashCounts = append(allCrashCounts, crashCounts...)
	}

	err = analyzer.store.SaveCrashCounts(allCrashCounts...)
	if err != nil {
		analyzer.logger.Error("Analyzer failed to save crash counts", err)
		return nil, err
	}

	err = analyzer.store.SavePendingStartMessages(allStartMessages...)
	if err != nil {
		analyzer.logger.Error("Analyzer failed to enqueue start messages", err)
		return nil, err
	}

	err = analyzer.store.SavePendingStopMessages(allStopMessages...)
	if err != nil {
		analyzer.logger.Error("Analyzer failed to enqueue stop messages", err)
		return nil, err
	}

	return actualApps, nil
}
