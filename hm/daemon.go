package hm

import (
	"errors"
	"time"
)

func Daemonize(callback func() error, period time.Duration, timeout time.Duration) error {
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
				return err
			}
		case <-timeoutChan:
			return errors.New("Daemon timed out")
		}
		<-afterChan
	}
	return nil
}
