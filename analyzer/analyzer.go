package analyzer

import (
	"errors"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/outbox"
	"github.com/cloudfoundry/hm9000/helpers/storecache"
	"github.com/cloudfoundry/hm9000/helpers/timeprovider"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
)

type Analyzer struct {
	store      store.Store
	storecache *storecache.StoreCache

	outbox       outbox.Outbox
	timeProvider timeprovider.TimeProvider
	conf         config.Config
}

func New(store store.Store, outbox outbox.Outbox, timeProvider timeprovider.TimeProvider, conf config.Config) *Analyzer {
	return &Analyzer{
		store:        store,
		outbox:       outbox,
		timeProvider: timeProvider,
		conf:         conf,
		storecache:   storecache.New(store),
	}
}

func (analyzer *Analyzer) Analyze() error {
	err := analyzer.verifyFreshness()
	if err != nil {
		return err
	}

	err = analyzer.storecache.Load()
	if err != nil {
		return err
	}

	allStartMessages := []models.QueueStartMessage{}
	allStopMessages := []models.QueueStopMessage{}

	for appVersionKey := range analyzer.storecache.SetOfApps {
		desired := analyzer.storecache.DesiredByApp[appVersionKey]
		runningInstances, hasRunning := analyzer.storecache.RunningByApp[appVersionKey]
		if !hasRunning {
			runningInstances = []models.InstanceHeartbeat{}
		}
		startMessages, stopMessages := analyzer.analyzeApp(desired, runningInstances)
		allStartMessages = append(allStartMessages, startMessages...)
		allStopMessages = append(allStopMessages, stopMessages...)
	}

	analyzer.outbox.Enqueue(allStartMessages, allStopMessages)
	return nil
}

func (analyzer *Analyzer) verifyFreshness() error {
	fresh, err := analyzer.store.IsDesiredStateFresh()
	if err != nil {
		return err
	}
	if !fresh {
		return errors.New("Desired state is not fresh")
	}

	fresh, err = analyzer.store.IsActualStateFresh(analyzer.timeProvider.Time())
	if err != nil {
		return err
	}
	if !fresh {
		return errors.New("Actual state is not fresh")
	}

	return nil
}
