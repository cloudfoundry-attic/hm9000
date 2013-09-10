package logger

import (
	"encoding/json"
	"log"
	"log/syslog"
	"os"
)

type Logger interface {
	Info(subject string, message map[string]string)
}

type RealLogger struct {
	logger        *log.Logger
	infoSysLogger *log.Logger
}

func NewRealLogger() *RealLogger {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	infoSysLogger, _ := syslog.NewLogger(syslog.LOG_INFO, log.LstdFlags)

	return &RealLogger{
		logger:        logger,
		infoSysLogger: infoSysLogger,
	}
}

func (logger *RealLogger) Info(subject string, message map[string]string) {
	messageString := "No Message"

	if message != nil {
		messageBytes, _ := json.Marshal(message)
		messageString = string(messageBytes)
	}

	logger.logger.Printf("%s - %s", subject, messageString)
	logger.infoSysLogger.Printf("%s - %s", subject, messageString)
}
