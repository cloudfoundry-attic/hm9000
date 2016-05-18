package models

import "sync"

type AppQueue struct {
	DesiredApps             chan map[string]DesiredAppState
	fetchDesiredAppsSuccess bool
	DoneAnalyzing           chan struct{}
	mutex                   *sync.Mutex
}

func NewAppQueue() *AppQueue {
	return &AppQueue{
		DesiredApps:             make(chan map[string]DesiredAppState, 1),
		fetchDesiredAppsSuccess: false,
		DoneAnalyzing:           make(chan struct{}),
		mutex:                   &sync.Mutex{},
	}
}

func (a *AppQueue) SetFetchDesiredAppsSuccess(val bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.fetchDesiredAppsSuccess = val
}

func (a *AppQueue) FetchDesiredAppsSuccess() bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.fetchDesiredAppsSuccess
}
