package shredder

import (
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/pivotal-golang/lager"
)

type Shredder struct {
	store  storepackage.Store
	logger lager.Logger
}

func New(store storepackage.Store, logger lager.Logger) *Shredder {
	return &Shredder{store, logger}
}

func (s *Shredder) Shred() error {
	return s.store.Compact()
}
