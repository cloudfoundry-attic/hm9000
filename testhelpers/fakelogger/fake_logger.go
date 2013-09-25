package fakelogger

import (
	"encoding/json"
	"fmt"
)

type FakeLogger struct {
	LoggedSubjects []string
	LoggedMessages []map[string]string
}

func NewFakeLogger() *FakeLogger {
	return &FakeLogger{
		LoggedSubjects: []string{},
		LoggedMessages: []map[string]string{},
	}
}

func (logger *FakeLogger) Info(subject string, message map[string]string) {
	_, err := json.Marshal(message)
	if err != nil {
		panic(fmt.Sprintf("LOGGER GOT AN UNMARSHALABLE MESSAGE: %s", err.Error()))
	}
	logger.LoggedSubjects = append(logger.LoggedSubjects, subject)
	logger.LoggedMessages = append(logger.LoggedMessages, message)
}
