package analyzer

import (
	"errors"
	"fmt"
	"os"
	"time"

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

func (analyzer *Analyzer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)
	for {
		afterChan := time.After(analyzer.conf.AnalyzerPollingInterval())
		timeoutChan := time.After(analyzer.conf.AnalyzerTimeout())
		errorChan := make(chan error, 1)

		t := time.Now()

		go func() {
			apps, err := analyzer.Analyze()
			SendMetrics(apps, err)
			errorChan <- err
		}()

		select {
		case err := <-errorChan:
			analyzer.logger.Info("ifrit time", lager.Data{
				"Component": "analyzer",
				"Duration":  fmt.Sprintf("%.4f", time.Since(t).Seconds()),
			})
			if err != nil {
				analyzer.logger.Error("Analyzer returned an error. Continuing...", err)
			}
		case <-timeoutChan:
			return errors.New("Analyzer timed out. Aborting!")
		case <-signals:
			return nil
		}

		<-afterChan
	}
}

func (analyzer *Analyzer) Analyze() (map[string]*models.App, error) {
	err := analyzer.store.VerifyFreshness(analyzer.clock.Now())
	if err != nil {
		analyzer.logger.Error("Store is not fresh", err)
		return nil, err
	}

	apps, err := analyzer.store.GetApps()
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

	for _, app := range apps {
		startMessages, stopMessages, crashCounts := newAppAnalyzer(app, analyzer.clock.Now(), existingPendingStartMessages, existingPendingStopMessages, analyzer.logger, analyzer.conf).analyzeApp()
		for _, startMessage := range startMessages {
			allStartMessages = append(allStartMessages, startMessage)
		}
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

	return apps, nil
}
