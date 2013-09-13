package models

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Heartbeat", func() {
	var heartbeat Heartbeat

	BeforeEach(func() {
		heartbeat = Heartbeat{
			DeaGuid: "dea_abc",
			InstanceHeartbeats: []InstanceHeartbeat{
				InstanceHeartbeat{
					CCPartition:    "default",
					AppGuid:        "abc",
					AppVersion:     "xyz-123",
					InstanceGuid:   "def",
					InstanceIndex:  3,
					State:          InstanceStateRunning,
					StateTimestamp: 1123.2,
				},
			},
		}
	})

	Describe("Building from JSON", func() {
		Context("When all is well", func() {
			It("should, like, totally build from JSON", func() {
				jsonHeartbeat, err := NewHeartbeatFromJSON([]byte(`{
                    "dea":"dea_abc",
                    "droplets":[
                        {
                            "cc_partition":"default",
                            "droplet":"abc",
                            "version":"xyz-123",
                            "instance":"def",
                            "index":3,
                            "state":"RUNNING",
                            "state_timestamp":1123.2
                        }
                    ]
                }`))

				Ω(err).ShouldNot(HaveOccured())

				Ω(jsonHeartbeat).Should(Equal(heartbeat))
			})
		})

		Context("When the JSON is invalid", func() {
			It("returns a zero heartbeat and an error", func() {
				heartbeat, err := NewHeartbeatFromJSON([]byte(`{`))

				Ω(heartbeat).Should(BeZero())
				Ω(err).Should(HaveOccured())
			})
		})
	})

	Describe("ToJson", func() {
		It("should, like, totally encode JSON", func() {
			jsonHeartbeat, err := NewHeartbeatFromJSON([]byte(heartbeat.ToJson()))

			Ω(err).ShouldNot(HaveOccured())
			Ω(jsonHeartbeat).Should(Equal(heartbeat))
		})
	})
})

var _ = Describe("InstanceHeartbeat", func() {
	var instance InstanceHeartbeat

	BeforeEach(func() {
		instance = InstanceHeartbeat{
			CCPartition:    "default",
			AppGuid:        "abc",
			AppVersion:     "xyz-123",
			InstanceGuid:   "def",
			InstanceIndex:  3,
			State:          InstanceStateRunning,
			StateTimestamp: 1123.2,
		}
	})

	Describe("Building from JSON", func() {
		Context("When all is well", func() {
			It("should, like, totally build from JSON", func() {
				jsonInstance, err := NewInstanceHeartbeatFromJSON([]byte(`{
                    "cc_partition":"default",
                    "droplet":"abc",
                    "version":"xyz-123",
                    "instance":"def",
                    "index":3,
                    "state":"RUNNING",
                    "state_timestamp":1123.2
                }`))

				Ω(err).ShouldNot(HaveOccured())

				Ω(jsonInstance).Should(Equal(instance))
			})
		})

		Context("When the JSON is invalid", func() {
			It("returns a zero heartbeat and an error", func() {
				instance, err := NewInstanceHeartbeatFromJSON([]byte(`{`))

				Ω(instance).Should(BeZero())
				Ω(err).Should(HaveOccured())
			})
		})
	})

	Describe("ToJson", func() {
		It("should, like, totally encode JSON", func() {
			jsonInstance, err := NewInstanceHeartbeatFromJSON([]byte(instance.ToJson()))

			Ω(err).ShouldNot(HaveOccured())
			Ω(jsonInstance).Should(Equal(instance))
		})
	})
})
