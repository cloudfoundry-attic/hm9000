package models

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("BasicAuthInfo", func() {
	It("outputs to JSON", func() {
		appState := DesiredAppState{
			AppGuid:           "myAppId",
			AppVersion:        "123",
			NumberOfInstances: 1,
			Memory:            1024,
			State:             AppStateStarted,
			PackageState:      AppPackageStatePending,
			UpdatedAt:         time.Unix(0, 0),
		}
		Ω(appState.ToJson()).Should(Equal(`{"id":"myAppId","version":"123","instances":1,"memory":1024,"state":"STARTED","package_state":"PENDING","updated_at":"1969-12-31T16:00:00-08:00"}`))
	})
})
