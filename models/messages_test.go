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
					AppGuid:       "abc",
					AppVersion:    "123",
					InstanceIndex: 1,
					RunningIndices: []RunningIndex{
						RunningIndex{0, 1},
						RunningIndex{1, 0},
						RunningIndex{2, 2},
					},
				}
				json := string(message.ToJSON())
				Ω(json).Should(ContainSubstring(`"droplet":"abc"`))
				Ω(json).Should(ContainSubstring(`"version":"123"`))
				Ω(json).Should(ContainSubstring(`"instance_index":1`))
				Ω(json).Should(ContainSubstring(`"running_indices":[{"index":0,"running":1},{"index":1,"running":0},{"index":2,"running":2}]`))
			})
		})
	})

	Describe("StopMessages", func() {
		Describe("ToJSON", func() {
			It("should have the right fields", func() {
				message := StopMessage{
					AppGuid:       "abc",
					AppVersion:    "123",
					InstanceGuid:  "def",
					InstanceIndex: 1,
					RunningIndices: []RunningIndex{
						RunningIndex{0, 1},
						RunningIndex{1, 3},
					},
				}
				json := string(message.ToJSON())
				Ω(json).Should(ContainSubstring(`"droplet":"abc"`))
				Ω(json).Should(ContainSubstring(`"version":"123"`))
				Ω(json).Should(ContainSubstring(`"instance_guid":"def"`))
				Ω(json).Should(ContainSubstring(`"instance_index":1`))
				Ω(json).Should(ContainSubstring(`"running_indices":[{"index":0,"running":1},{"index":1,"running":3}]`))
			})
		})
	})
})
