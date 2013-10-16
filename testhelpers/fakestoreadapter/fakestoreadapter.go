package fakestoreadapter

import (
	"github.com/cloudfoundry/hm9000/storeadapter"
	"regexp"
	"strings"
)

type FakeStoreAdapterErrorInjector struct {
	KeyRegexp *regexp.Regexp
	Error     error
}

func NewFakeStoreAdapterErrorInjector(keyRegexp string, err error) *FakeStoreAdapterErrorInjector {
	return &FakeStoreAdapterErrorInjector{
		KeyRegexp: regexp.MustCompile(keyRegexp),
		Error:     err,
	}
}

type FakeStoreAdapter struct {
	DidConnect    bool
	DidDisconnect bool

	ConnectErr        error
	DisconnectErr     error
	SetErrInjector    *FakeStoreAdapterErrorInjector
	GetErrInjector    *FakeStoreAdapterErrorInjector
	ListErrInjector   *FakeStoreAdapterErrorInjector
	DeleteErrInjector *FakeStoreAdapterErrorInjector

	store map[string]storeadapter.StoreNode
}

func New() *FakeStoreAdapter {
	adapter := &FakeStoreAdapter{}
	adapter.Reset()
	return adapter
}

func (adapter *FakeStoreAdapter) Reset() {
	adapter.DidConnect = false
	adapter.DidDisconnect = false

	adapter.ConnectErr = nil
	adapter.DisconnectErr = nil
	adapter.SetErrInjector = nil
	adapter.GetErrInjector = nil
	adapter.ListErrInjector = nil
	adapter.DeleteErrInjector = nil

	adapter.store = make(map[string]storeadapter.StoreNode)
}

func (adapter *FakeStoreAdapter) Connect() error {
	adapter.DidConnect = true
	return adapter.ConnectErr
}

func (adapter *FakeStoreAdapter) Disconnect() error {
	adapter.DidDisconnect = true
	return adapter.DisconnectErr
}

func (adapter *FakeStoreAdapter) Set(nodes []storeadapter.StoreNode) error {
	for _, node := range nodes {
		if adapter.SetErrInjector != nil && adapter.SetErrInjector.KeyRegexp.MatchString(node.Key) {
			return adapter.SetErrInjector.Error
		}
		adapter.store[node.Key] = node
	}
	return nil
}

func (adapter *FakeStoreAdapter) Get(key string) (storeadapter.StoreNode, error) {
	if adapter.GetErrInjector != nil && adapter.GetErrInjector.KeyRegexp.MatchString(key) {
		return storeadapter.StoreNode{}, adapter.GetErrInjector.Error
	}
	node, present := adapter.store[key]
	if !present {
		return storeadapter.StoreNode{}, storeadapter.ErrorKeyNotFound
	}
	return node, nil
}

func (adapter *FakeStoreAdapter) List(key string) (results []storeadapter.StoreNode, err error) {
	if adapter.ListErrInjector != nil && adapter.ListErrInjector.KeyRegexp.MatchString(key) {
		return []storeadapter.StoreNode{}, adapter.ListErrInjector.Error
	}

	for nodeKey, node := range adapter.store {
		if strings.HasPrefix(nodeKey, key+"/") {
			results = append(results, node)
		}
	}

	return results, nil
}

func (adapter *FakeStoreAdapter) Delete(key string) error {
	if adapter.DeleteErrInjector != nil && adapter.DeleteErrInjector.KeyRegexp.MatchString(key) {
		return adapter.DeleteErrInjector.Error
	}

	_, present := adapter.store[key]
	if !present {
		return storeadapter.ErrorKeyNotFound
	}
	delete(adapter.store, key)
	return nil
}
