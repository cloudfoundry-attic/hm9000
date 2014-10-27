package hm

import (
	"fmt"
	"os"

	"github.com/cloudfoundry/hm9000/helpers/logger"
	"github.com/pivotal-golang/lager"
)

type LagerAdapter struct {
	oldLogger logger.Logger
}

func (_ *LagerAdapter) RegisterSink(_ lager.Sink) {}

func (l *LagerAdapter) Session(task string, data ...lager.Data) lager.Logger {
	return l
}

func (l *LagerAdapter) Debug(action string, data ...lager.Data) {
	l.oldLogger.Debug(action, dataToStrings(data)...)
}

func (l *LagerAdapter) Info(action string, data ...lager.Data) {
	l.oldLogger.Info(action, dataToStrings(data)...)
}

func (l *LagerAdapter) Error(action string, err error, data ...lager.Data) {
	l.oldLogger.Error(action, err, dataToStrings(data)...)
}

func (l *LagerAdapter) Fatal(action string, err error, data ...lager.Data) {
	l.oldLogger.Error(action, err, dataToStrings(data)...)
	os.Exit(1)
}

func (l *LagerAdapter) WithData(lager.Data) lager.Logger {
	return l
}

func dataToStrings(data []lager.Data) []map[string]string {
	stringMaps := []map[string]string{}
	for _, item := range data {
		stringMap := make(map[string]string)
		for k, v := range item {
			stringMap[k] = fmt.Sprintf("%v", v)
		}
		stringMaps = append(stringMaps, stringMap)
	}
	return stringMaps
}
