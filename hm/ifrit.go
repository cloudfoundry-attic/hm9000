package hm

import (
	"fmt"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
	"os"
)

func ifritize(
	component string,
	conf *config.Config,
	runner ifrit.Runner,
	logger lager.Logger,
) error {
	releaseLockChannel := acquireLock(logger, conf, component)

	group := grouper.NewOrdered(os.Interrupt, grouper.Members{
		{component, runner},
	})
	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info(fmt.Sprintf("%s ifrit started", component))

	err := <-monitor.Wait()
	if err != nil {
		logger.Error(fmt.Sprintf("%s exiting on error", component), err)
	}
	released := make(chan bool)
	releaseLockChannel <- released
	<-released
	return err
}
