package fakelogger

import (
	"encoding/json"
	"fmt"
	"sync"
)

type FakeLogger struct {
	mutex          sync.Mutex
	LoggedSubjects []string
	LoggedErrors   []error
	LoggedMessages []string
}

func NewFakeLogger() *FakeLogger {
	return &FakeLogger{
		LoggedSubjects: []string{},
		LoggedErrors:   []error{},
		LoggedMessages: []string{},
	}
}

func (logger *FakeLogger) Info(subject string, messages ...map[string]string) {
	logger.mutex.Lock()
	logger.LoggedSubjects = append(logger.LoggedSubjects, subject)
	logger.LoggedMessages = append(logger.LoggedMessages, logger.squashedMessage(messages...))
	logger.mutex.Unlock()
}

func (logger *FakeLogger) Debug(subject string, messages ...map[string]string) {
	logger.mutex.Lock()
	logger.LoggedSubjects = append(logger.LoggedSubjects, subject)
	logger.LoggedMessages = append(logger.LoggedMessages, logger.squashedMessage(messages...))
	logger.mutex.Unlock()
}

func (logger *FakeLogger) Error(subject string, err error, messages ...map[string]string) {
	logger.mutex.Lock()
	logger.LoggedSubjects = append(logger.LoggedSubjects, subject)
	logger.LoggedErrors = append(logger.LoggedErrors, err)
	logger.LoggedMessages = append(logger.LoggedMessages, logger.squashedMessage(messages...))
	logger.mutex.Unlock()
}

func (logger *FakeLogger) squashedMessage(messages ...map[string]string) (squashed string) {
	for _, message := range messages {
		encoded, err := json.Marshal(message)
		if err != nil {
			panic(fmt.Sprintf("LOGGER GOT AN UNMARSHALABLE MESSAGE: %s", err.Error()))
		}
		squashed += " - " + string(encoded)
	}
	return
}
