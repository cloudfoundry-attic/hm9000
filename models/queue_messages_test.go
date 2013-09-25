package models_test

import (
	. "github.com/cloudfoundry/hm9000/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"time"
)

var _ = Describe("QueueMessages", func() {
	Describe("Start Message", func() {
		var message QueueStartMessage
		BeforeEach(func() {
			message = NewQueueStartMessage(time.Unix(100, 0), 30, 10, "app-guid", "app-version", 1)
		})

		Describe("Creating new start messages programatically", func() {
			It("should populate the start message correctly, and compute the correct SendOn time", func() {
				Ω(message.SendOn).Should(BeNumerically("==", 130))
				Ω(message.SentOn).Should(BeNumerically("==", 0))
				Ω(message.KeepAlive).Should(BeNumerically("==", 10))
				Ω(message.AppGuid).Should(Equal("app-guid"))
				Ω(message.AppVersion).Should(Equal("app-version"))
				Ω(message.IndexToStart).Should(Equal(1))
			})
		})

		Describe("Creating new start messages from JSON", func() {
			Context("when passed valid JSON", func() {
				It("should parse correctly", func() {
					parsed, err := NewQueueStartMessageFromJSON([]byte(`{
                        "send_on": 130,
                        "sent_on": 0,
                        "keep_alive": 10,
                        "droplet": "app-guid",
                        "version": "app-version",
                        "index": 1
                    }`))
					Ω(err).ShouldNot(HaveOccured())
					Ω(parsed).Should(Equal(message))
				})
			})

			Context("when passed unparseable JSON", func() {
				It("should error", func() {
					parsed, err := NewQueueStartMessageFromJSON([]byte(`ß`))
					Ω(parsed).Should(BeZero())
					Ω(err).Should(HaveOccured())
				})
			})
		})

		Describe("ToJSON", func() {
			It("should generate valid JSON", func() {
				roundTripMessage, err := NewQueueStartMessageFromJSON(message.ToJSON())
				Ω(err).ShouldNot(HaveOccured())
				Ω(roundTripMessage).Should(Equal(message))
			})
		})

		Describe("StoreKey", func() {
			It("should generate the correct key", func() {
				Ω(message.StoreKey()).Should(Equal("app-guid-app-version-1"))
			})
		})

		Describe("LogDescription", func() {
			It("should generate an appropriate map", func() {
				Ω(message.LogDescription()).Should(Equal(map[string]string{
					"SendOn":       time.Unix(130, 0).String(),
					"SentOn":       time.Unix(0, 0).String(),
					"KeepAlive":    "10",
					"AppGuid":      "app-guid",
					"AppVersion":   "app-version",
					"IndexToStart": "1",
				}))
			})
		})
	})

	Describe("Stop Message", func() {
		var message QueueStopMessage
		BeforeEach(func() {
			message = NewQueueStopMessage(time.Unix(100, 0), 30, 10, "instance-guid")
		})

		Describe("Creating new start messages programatically", func() {
			It("should populate the start message correctly, and compute the correct SendOn time", func() {
				Ω(message.SendOn).Should(BeNumerically("==", 130))
				Ω(message.SentOn).Should(BeNumerically("==", 0))
				Ω(message.KeepAlive).Should(BeNumerically("==", 10))
				Ω(message.InstanceGuid).Should(Equal("instance-guid"))
			})
		})

		Describe("Creating new start messages from JSON", func() {
			Context("when passed valid JSON", func() {
				It("should parse correctly", func() {
					parsed, err := NewQueueStopMessageFromJSON([]byte(`{
                        "send_on": 130,
                        "sent_on": 0,
                        "keep_alive": 10,
                        "instance": "instance-guid"
                    }`))
					Ω(err).ShouldNot(HaveOccured())
					Ω(parsed).Should(Equal(message))
				})
			})

			Context("when passed unparseable JSON", func() {
				It("should error", func() {
					parsed, err := NewQueueStopMessageFromJSON([]byte(`ß`))
					Ω(parsed).Should(BeZero())
					Ω(err).Should(HaveOccured())
				})
			})
		})

		Describe("ToJSON", func() {
			It("should generate valid JSON", func() {
				roundTripMessage, err := NewQueueStopMessageFromJSON(message.ToJSON())
				Ω(err).ShouldNot(HaveOccured())
				Ω(roundTripMessage).Should(Equal(message))
			})
		})

		Describe("StoreKey", func() {
			It("should generate the correct key", func() {
				Ω(message.StoreKey()).Should(Equal("instance-guid"))
			})
		})

		Describe("LogDescription", func() {
			It("should generate an appropriate map", func() {
				Ω(message.LogDescription()).Should(Equal(map[string]string{
					"SendOn":       time.Unix(130, 0).String(),
					"SentOn":       time.Unix(0, 0).String(),
					"KeepAlive":    "10",
					"InstanceGuid": "instance-guid",
				}))
			})
		})
	})
})
