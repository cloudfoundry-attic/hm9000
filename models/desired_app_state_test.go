package models

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"encoding/json"
	"time"
)

var _ = Describe("DesiredAppState", func() {
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

		var decoded DesiredAppState
		err := json.Unmarshal(appState.ToJson(), &decoded)
		Ω(err).ShouldNot(HaveOccured())
		Ω(decoded).Should(EqualDesiredState(appState))
	})

	Describe("Equality", func() {
		var (
			actual DesiredAppState
			other  DesiredAppState
		)
		BeforeEach(func() {
			actual = DesiredAppState{
				AppGuid:           "a guid",
				AppVersion:        "a version",
				NumberOfInstances: 1,
				Memory:            256,
				State:             AppStateStarted,
				PackageState:      AppPackageStateStaged,
				UpdatedAt:         time.Unix(0, 0),
			}

			other = actual
		})

		It("is equal when all fields are equal", func() {
			Ω(actual.Equal(other)).Should(BeTrue())
		})

		It("is inequal when the app guid is different", func() {
			other.AppGuid = "not an app guid"
			Ω(actual.Equal(other)).Should(BeFalse())
		})

		It("is inequal when the app version is different", func() {
			other.AppVersion = "not an app version"
			Ω(actual.Equal(other)).Should(BeFalse())
		})

		It("is inequal when the number of instances is different", func() {
			other.NumberOfInstances = 9000
			Ω(actual.Equal(other)).Should(BeFalse())
		})

		It("is inequal when the memory is different", func() {
			other.Memory = 4096
			Ω(actual.Equal(other)).Should(BeFalse())
		})

		It("is inequal when the state is different", func() {
			other.State = AppStateStopped
			Ω(actual.Equal(other)).Should(BeFalse())
		})

		It("is inequal when the package state is different", func() {
			other.PackageState = AppPackageStateFailed
			Ω(actual.Equal(other)).Should(BeFalse())
		})

		It("is inequal when the updated at is different", func() {
			other.UpdatedAt = time.Unix(9000, 9000)
			Ω(actual.Equal(other)).Should(BeFalse())
		})
	})
})
