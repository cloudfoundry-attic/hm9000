package hm

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

type Component struct {
	component       string
	conf            *config.Config
	pollingInterval time.Duration
	timeout         time.Duration
	logger          lager.Logger

	action func() error
}

func NewComponent(name string, conf *config.Config, pollingInterval time.Duration, timeout time.Duration,
	logger lager.Logger, action func() error) *Component {
	return &Component{
		name,
		conf,
		pollingInterval,
		timeout,
		logger,
		action,
	}
}

func ifritizeComponent(hm *Component) error {
	return ifritize(hm.logger, hm.component, hm, hm.conf)
}

func ifritizeNoLock(logger lager.Logger, name string, runner ifrit.Runner, conf *config.Config) error {
	group := grouper.NewOrdered(os.Interrupt, grouper.Members{
		{name, runner},
	})
	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info(fmt.Sprintf("%s started", name))

	err := <-monitor.Wait()
	if err != nil {
		logger.Error(fmt.Sprintf("%s exiting on error", name), err)
	}

	return err
}

func ifritize(logger lager.Logger, name string, runner ifrit.Runner, conf *config.Config) error {
	releaseLockChannel := acquireLock(logger, conf, name)

	err := ifritizeNoLock(logger, name, runner, conf)

	released := make(chan bool)
	releaseLockChannel <- released
	<-released
	return err
}

func (hm *Component) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	close(ready)
	for {
		afterChan := time.After(hm.pollingInterval)
		timeoutTicker := time.NewTicker(hm.timeout)
		errorChan := make(chan error, 1)

		go func() {
			errorChan <- hm.action()
		}()

		select {
		case <-errorChan:

		case <-timeoutTicker.C:
			return errors.New(hm.component + " timed out. Aborting!")
		case <-signals:
			return nil
		}

		timeoutTicker.Stop()
		select {
		case <-signals:
			return nil

		case <-afterChan:
		}
	}
}
