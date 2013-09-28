package models_test

import (
	. "github.com/cloudfoundry/hm9000/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Messages", func() {
	Describe("StartMessages", func() {
		Describe("ToJSON", func() {
			It("should have the right fields", func() {
				message := StartMessage{
					AppGuid:        "abc",
					AppVersion:     "123",
					InstanceIndex:  1,
					RunningIndices: RunningIndices{"0": 1, "1": 0, "2": 2},
				}
				json := string(message.ToJSON())
				Ω(json).Should(ContainSubstring(`"droplet":"abc"`))
				Ω(json).Should(ContainSubstring(`"version":"123"`))
				Ω(json).Should(ContainSubstring(`"instance_index":1`))
				Ω(json).Should(ContainSubstring(`"running_indices":{"0":1,"1":0,"2":2}`))
			})
		})
		Describe("NewStartMessageFromJSON", func() {
			It("should create right start message", func() {
				message := StartMessage{
					AppGuid:        "abc",
					AppVersion:     "123",
					InstanceIndex:  1,
					RunningIndices: RunningIndices{"0": 1, "1": 0, "2": 2},
				}
				decodedMessage, err := NewStartMessageFromJSON(message.ToJSON())
				Ω(err).ShouldNot(HaveOccured())
				Ω(decodedMessage).Should(Equal(message))
			})

			It("should error when passed invalid json", func() {
				message, err := NewStartMessageFromJSON([]byte("∂"))
				Ω(message).Should(BeZero())
				Ω(err).Should(HaveOccured())
			})
		})
	})

	Describe("StopMessages", func() {
		Describe("ToJSON", func() {
			It("should have the right fields", func() {
				message := StopMessage{
					AppGuid:        "abc",
					AppVersion:     "123",
					InstanceGuid:   "def",
					InstanceIndex:  1,
					RunningIndices: RunningIndices{"0": 1, "1": 3},
				}
				json := string(message.ToJSON())
				Ω(json).Should(ContainSubstring(`"droplet":"abc"`))
				Ω(json).Should(ContainSubstring(`"version":"123"`))
				Ω(json).Should(ContainSubstring(`"instance_guid":"def"`))
				Ω(json).Should(ContainSubstring(`"instance_index":1`))
				Ω(json).Should(ContainSubstring(`"running_indices":{"0":1,"1":3}`))
			})
		})
		Describe("NewStopMessageFromJSON", func() {
			It("should create right stop message", func() {
				message := StopMessage{
					AppGuid:        "abc",
					AppVersion:     "123",
					InstanceGuid:   "def",
					InstanceIndex:  1,
					RunningIndices: RunningIndices{"0": 1, "1": 3},
				}
				decodedMessage, err := NewStopMessageFromJSON(message.ToJSON())
				Ω(err).ShouldNot(HaveOccured())
				Ω(decodedMessage).Should(Equal(message))
			})

			It("should error when passed invalid json", func() {
				message, err := NewStopMessageFromJSON([]byte("∂"))
				Ω(message).Should(BeZero())
				Ω(err).Should(HaveOccured())
			})
		})
	})

	Describe("IncrementIndex", func() {
		It("should increment index of running indices", func() {
			runningIndices := RunningIndices{"0": 1, "1": 2}

			runningIndices.IncrementIndex(0)

			Ω(runningIndices).Should(Equal(RunningIndices{"0": 2, "1": 2}))

			runningIndices.IncrementIndex(3)

			Ω(runningIndices).Should(Equal(RunningIndices{"0": 2, "1": 2, "3": 1}))
		})
	})
})
