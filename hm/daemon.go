package hm

import (
	"errors"
	"fmt"
	"github.com/cloudfoundry/hm9000/helpers/logger"
	"time"
)

func Daemonize(callback func() error, period time.Duration, timeout time.Duration, l logger.Logger) error {
	l.Info(fmt.Sprintf("Running Daemon every %d seconds with a timeout of %d", int(period.Seconds()), int(timeout.Seconds())))
	for true {
		afterChan := time.After(period)
		timeoutChan := time.After(timeout)
		errorChan := make(chan error, 1)
		go func() {
			errorChan <- callback()
		}()
		select {
		case err := <-errorChan:
			if err != nil {
				l.Error("Daemon returned an error. Continuining...", err)
			}
		case <-timeoutChan:
			return errors.New("Daemon timed out. Aborting!")
		}
		<-afterChan
	}
	return nil
}
