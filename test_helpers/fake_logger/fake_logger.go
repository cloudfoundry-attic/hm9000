package fake_logger

import (
	"encoding/json"
	"fmt"
)

type FakeLogger struct{}

func NewFakeLogger() *FakeLogger {
	return &FakeLogger{}
}

func (logger *FakeLogger) Info(subject string, message map[string]string) {
	_, err := json.Marshal(message)
	if err != nil {
		panic(fmt.Sprintf("LOGGER GOT AN UNMARSHALABLE MESSAGE: %s", err.Error()))
	}
}
