package fakelogger

import (
	"encoding/json"
	"fmt"
)

type FakeLogger struct {
	LoggedSubjects []string
}

func NewFakeLogger() *FakeLogger {
	return &FakeLogger{
		LoggedSubjects: []string{},
	}
}

func (logger *FakeLogger) Info(subject string, message map[string]string) {
	_, err := json.Marshal(message)
	if err != nil {
		panic(fmt.Sprintf("LOGGER GOT AN UNMARSHALABLE MESSAGE: %s", err.Error()))
	}
	logger.LoggedSubjects = append(logger.LoggedSubjects, subject)
}
