package fakelogger

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/pivotal-golang/lager"
)

type FakeLogger struct {
	mutex          sync.Mutex
	loggedSubjects []string
	loggedErrors   []error
	loggedMessages []string
	task           string
	data           lager.Data
}

func NewFakeLogger() *FakeLogger {
	return &FakeLogger{
		loggedSubjects: []string{},
		loggedErrors:   []error{},
		loggedMessages: []string{},
	}
}

func (logger *FakeLogger) Info(subject string, messages ...lager.Data) {
	logger.mutex.Lock()
	logger.loggedSubjects = append(logger.loggedSubjects, subject)
	logger.loggedMessages = append(logger.loggedMessages, logger.squashedMessage(messages...))
	logger.mutex.Unlock()
}

func (logger *FakeLogger) Debug(subject string, messages ...lager.Data) {
	logger.mutex.Lock()
	logger.loggedSubjects = append(logger.loggedSubjects, subject)
	logger.loggedMessages = append(logger.loggedMessages, logger.squashedMessage(messages...))
	logger.mutex.Unlock()
}

func (logger *FakeLogger) Error(subject string, err error, messages ...lager.Data) {
	logger.mutex.Lock()
	logger.loggedSubjects = append(logger.loggedSubjects, subject)
	logger.loggedErrors = append(logger.loggedErrors, err)
	logger.loggedMessages = append(logger.loggedMessages, logger.squashedMessage(messages...))
	logger.mutex.Unlock()
}

func (logger *FakeLogger) Fatal(subject string, err error, messages ...lager.Data) {
	logger.mutex.Lock()
	logger.loggedSubjects = append(logger.loggedSubjects, subject)
	logger.loggedErrors = append(logger.loggedErrors, err)
	logger.loggedMessages = append(logger.loggedMessages, logger.squashedMessage(messages...))
	logger.mutex.Unlock()
}

func (logger *FakeLogger) squashedMessage(messages ...lager.Data) (squashed string) {
	for _, message := range messages {
		encoded, err := json.Marshal(message)
		if err != nil {
			panic(fmt.Sprintf("LOGGER GOT AN UNMARSHALABLE MESSAGE: %s", err.Error()))
		}
		squashed += " - " + string(encoded)
	}
	return
}

func (logger *FakeLogger) RegisterSink(sink lager.Sink) {}

func (logger *FakeLogger) Session(task string, data ...lager.Data) lager.Logger {
	logger.task = task
	return logger
}

func (logger *FakeLogger) SessionName() string {
	return logger.task
}

func (logger *FakeLogger) WithData(data lager.Data) lager.Logger {
	return &FakeLogger{
		loggedSubjects: []string{},
		loggedErrors:   []error{},
		loggedMessages: []string{},
		data:           data,
	}
}

func (logger *FakeLogger) LoggedSubjects() []string {
	defer logger.mutex.Unlock()
	logger.mutex.Lock()
	return logger.loggedSubjects[:]
}

func (logger *FakeLogger) LoggedErrors() []error {
	defer logger.mutex.Unlock()
	logger.mutex.Lock()
	return logger.loggedErrors[:]
}

func (logger *FakeLogger) LoggedMessages() []string {
	defer logger.mutex.Unlock()
	logger.mutex.Lock()
	return logger.loggedMessages[:]
}
