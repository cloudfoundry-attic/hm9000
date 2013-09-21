package phd

import (
	"fmt"
	g "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"strings"
)

type StorePerformanceReport struct {
	StoreType     string
	NumStoreNodes int
	NumApps       int
	Subject       string
	Average       float64
	StdDeviation  float64
}

func (r StorePerformanceReport) String() string {
	return fmt.Sprintf("%s: %d %s node(s), %d apps", strings.Title(r.Subject), r.NumStoreNodes, r.StoreType, r.NumApps)
}

type DataReporter struct {
	writePerformanceReports     []StorePerformanceReport
	readPerformanceReports      []StorePerformanceReport
	diskUsagePerformanceReports []StorePerformanceReport
}

func (reporter *DataReporter) SpecSuiteWillBegin(config config.GinkgoConfigType, summary *g.SuiteSummary) {
	reporter.writePerformanceReports = make([]StorePerformanceReport, 0)
	reporter.readPerformanceReports = make([]StorePerformanceReport, 0)
	reporter.diskUsagePerformanceReports = make([]StorePerformanceReport, 0)
}

func (reporter *DataReporter) ExampleDidComplete(exampleSummary *g.ExampleSummary) {
	for _, measurement := range exampleSummary.Measurements {
		info := measurement.Info.(StorePerformanceReport)
		info.Average = measurement.Average
		info.StdDeviation = measurement.StdDeviation
		if info.Subject == "write performance" {
			reporter.writePerformanceReports = append(reporter.writePerformanceReports, info)
		} else if info.Subject == "read performance" {
			reporter.readPerformanceReports = append(reporter.readPerformanceReports, info)
		} else if info.Subject == "disk usage" {
			reporter.diskUsagePerformanceReports = append(reporter.diskUsagePerformanceReports, info)
		}
	}
}

func (reporter *DataReporter) SpecSuiteDidEnd(summary *g.SuiteSummary) {
	//todo: collate and print, then use the data reporter in the test suite
}
