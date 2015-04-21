package fakeregistrar

import "github.com/cloudfoundry/loggregatorlib/cfcomponent"

type FakeRegistrar struct {
	RegisteredWithCollector cfcomponent.Component
}

func New() *FakeRegistrar {
	return &FakeRegistrar{}
}

func (fr *FakeRegistrar) RegisterWithCollector(component cfcomponent.Component) error {
	fr.RegisteredWithCollector = component
	return nil
}
