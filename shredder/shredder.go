package shredder

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cloudfoundry/hm9000/config"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/pivotal-golang/lager"
)

type Shredder struct {
	store  storepackage.Store
	conf   *config.Config
	logger lager.Logger
}

func New(store storepackage.Store, conf *config.Config, logger lager.Logger) *Shredder {
	return &Shredder{store, conf, logger}
}

func (shredder *Shredder) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)
	for {
		afterChan := time.After(shredder.conf.ShredderPollingInterval())
		timeoutChan := time.After(shredder.conf.ShredderTimeout())
		errorChan := make(chan error, 1)

		t := time.Now()

		go func() {
			err := shredder.Shred()
			errorChan <- err
		}()

		select {
		case err := <-errorChan:
			shredder.logger.Info("ifrit time", lager.Data{
				"Component": "shredder",
				"Duration":  fmt.Sprintf("%.4f", time.Since(t).Seconds()),
			})
			if err != nil {
				shredder.logger.Error("Shredder returned an error. Continuing...", err)
			}
		case <-timeoutChan:
			return errors.New("Shredder timed out. Aborting!")
		case <-signals:
			return nil
		}

		<-afterChan
	}
}

func (s *Shredder) Shred() error {
	return s.store.Compact()
}
