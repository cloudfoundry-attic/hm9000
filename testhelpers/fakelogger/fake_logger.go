package fakelogger

import (
	"encoding/json"
	"fmt"
	"sync"
)

type FakeLogger struct {
	mutex          sync.Mutex
	loggedSubjects []string
	loggedErrors   []error
	loggedMessages []string
}

func NewFakeLogger() *FakeLogger {
	return &FakeLogger{
		loggedSubjects: []string{},
		loggedErrors:   []error{},
		loggedMessages: []string{},
	}
}

func (logger *FakeLogger) Info(subject string, messages ...map[string]string) {
	logger.mutex.Lock()
	logger.loggedSubjects = append(logger.loggedSubjects, subject)
	logger.loggedMessages = append(logger.loggedMessages, logger.squashedMessage(messages...))
	logger.mutex.Unlock()
}

func (logger *FakeLogger) Debug(subject string, messages ...map[string]string) {
	logger.mutex.Lock()
	logger.loggedSubjects = append(logger.loggedSubjects, subject)
	logger.loggedMessages = append(logger.loggedMessages, logger.squashedMessage(messages...))
	logger.mutex.Unlock()
}

func (logger *FakeLogger) Error(subject string, err error, messages ...map[string]string) {
	logger.mutex.Lock()
	logger.loggedSubjects = append(logger.loggedSubjects, subject)
	logger.loggedErrors = append(logger.loggedErrors, err)
	logger.loggedMessages = append(logger.loggedMessages, logger.squashedMessage(messages...))
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

func (logger *FakeLogger) LoggedSubjects() []string {
	defer logger.mutex.Unlock()
	logger.mutex.Lock()
	return logger.loggedSubjects[:]
}
