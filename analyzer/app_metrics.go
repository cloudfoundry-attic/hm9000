package analyzer

import (
	"github.com/cloudfoundry/dropsonde/metrics"
	"github.com/cloudfoundry/hm9000/models"
)

const (
	NumberOfAppsWithAllInstancesReporting = "NumberOfAppsWithAllInstancesReporting"
	NumberOfAppsWithMissingInstances      = "NumberOfAppsWithMissingInstances"
	NumberOfUndesiredRunningApps          = "NumberOfUndesiredRunningApps"
	NumberOfRunningInstances              = "NumberOfRunningInstances"
	NumberOfMissingIndices                = "NumberOfMissingIndices"
	NumberOfCrashedInstances              = "NumberOfCrashedInstances"
	NumberOfCrashedIndices                = "NumberOfCrashedIndices"
	NumberOfDesiredApps                   = "NumberOfDesiredApps"
	NumberOfDesiredInstances              = "NumberOfDesiredInstances"
	NumberOfDesiredAppsPendingStaging     = "NumberOfDesiredAppsPendingStaging"
)

func SendMetrics(apps map[string]*models.App, err error) {
	counts := map[string]int{
		NumberOfAppsWithAllInstancesReporting: 0,
		NumberOfAppsWithMissingInstances:      0,
		NumberOfUndesiredRunningApps:          0,
		NumberOfRunningInstances:              0,
		NumberOfMissingIndices:                0,
		NumberOfCrashedInstances:              0,
		NumberOfCrashedIndices:                0,
		NumberOfDesiredApps:                   0,
		NumberOfDesiredInstances:              0,
		NumberOfDesiredAppsPendingStaging:     0,
	}

	if err == nil {
		for _, app := range apps {
			if app.IsDesired() {
				if app.Desired.PackageState == models.AppPackageStatePending {
					addToCounter(counts, NumberOfDesiredAppsPendingStaging, 1)
				} else {
					addToCounter(counts, NumberOfDesiredApps, 1)
					addToCounter(counts, NumberOfDesiredInstances, app.NumberOfDesiredInstances())

					numberOfMissingIndicesForApp := app.NumberOfDesiredInstances() - app.NumberOfDesiredIndicesReporting()
					if numberOfMissingIndicesForApp == 0 {
						addToCounter(counts, NumberOfAppsWithAllInstancesReporting, 1)
					} else {
						addToCounter(counts, NumberOfAppsWithMissingInstances, 1)
						addToCounter(counts, NumberOfMissingIndices, numberOfMissingIndicesForApp)
					}
				}
			} else {
				if app.HasStartingOrRunningInstances() {
					addToCounter(counts, NumberOfUndesiredRunningApps, 1)
				}
			}

			addToCounter(counts, NumberOfRunningInstances, app.NumberOfStartingOrRunningInstances())
			addToCounter(counts, NumberOfCrashedInstances, app.NumberOfCrashedInstances())
			addToCounter(counts, NumberOfCrashedIndices, app.NumberOfCrashedIndices())
		}

		for key, val := range counts {
			metrics.SendValue(key, float64(val), "Metric")
		}
	} else {
		for key, _ := range counts {
			metrics.SendValue(key, float64(-1), "Metric")
		}
	}
}

func addToCounter(counts map[string]int, key string, increment int) {
	counts[key] = counts[key] + increment
}
