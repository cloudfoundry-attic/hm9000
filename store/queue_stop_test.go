package store_test

import (
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Storing QueueStopMessages", func() {
	var (
		store       Store
		etcdAdapter storeadapter.StoreAdapter
		conf        config.Config
		message1    models.QueueStopMessage
		message2    models.QueueStopMessage
		message3    models.QueueStopMessage
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())
		etcdAdapter = storeadapter.NewETCDStoreAdapter(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err = etcdAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())

		message1 = models.NewQueueStopMessage(time.Unix(100, 0), 10, 4, "ABC")
		message2 = models.NewQueueStopMessage(time.Unix(100, 0), 10, 4, "DEF")
		message3 = models.NewQueueStopMessage(time.Unix(100, 0), 10, 4, "GHI")

		store = NewStore(conf, etcdAdapter)
	})

	AfterEach(func() {
		etcdAdapter.Disconnect()
	})

	Describe("Saving stop messages", func() {
		BeforeEach(func() {
			err := store.SaveQueueStopMessages([]models.QueueStopMessage{
				message1,
				message2,
			})
			Ω(err).ShouldNot(HaveOccured())
		})

		It("stores the passed in stop messages", func() {
			nodes, err := etcdAdapter.List("/stop")
			Ω(err).ShouldNot(HaveOccured())
			Ω(nodes).Should(HaveLen(2))
			Ω(nodes).Should(ContainElement(storeadapter.StoreNode{
				Key:   "/stop/" + message1.StoreKey(),
				Value: message1.ToJSON(),
				TTL:   0,
			}))
			Ω(nodes).Should(ContainElement(storeadapter.StoreNode{
				Key:   "/stop/" + message2.StoreKey(),
				Value: message2.ToJSON(),
				TTL:   0,
			}))
		})
	})

	Describe("Fetching stop message", func() {
		Context("When the stop message is present", func() {
			BeforeEach(func() {
				err := store.SaveQueueStopMessages([]models.QueueStopMessage{
					message1,
					message2,
				})
				Ω(err).ShouldNot(HaveOccured())
			})

			It("can fetch the stop message", func() {
				desired, err := store.GetQueueStopMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(desired).Should(HaveLen(2))
				Ω(desired).Should(ContainElement(message1))
				Ω(desired).Should(ContainElement(message2))
			})
		})

		Context("when the stop message is empty", func() {
			BeforeEach(func() {
				hb := message1
				err := store.SaveQueueStopMessages([]models.QueueStopMessage{hb})
				Ω(err).ShouldNot(HaveOccured())
				err = store.DeleteQueueStopMessages([]models.QueueStopMessage{hb})
				Ω(err).ShouldNot(HaveOccured())
			})

			It("returns an empty array", func() {
				stop, err := store.GetQueueStopMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(stop).Should(BeEmpty())
			})
		})

		Context("When the stop message key is missing", func() {
			BeforeEach(func() {
				_, err := etcdAdapter.List("/stop")
				Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))
			})

			It("returns an empty array and no error", func() {
				stop, err := store.GetQueueStopMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(stop).Should(BeEmpty())
			})
		})
	})

	Describe("Deleting stop message", func() {
		BeforeEach(func() {
			err := store.SaveQueueStopMessages([]models.QueueStopMessage{
				message1,
				message2,
				message3,
			})
			Ω(err).ShouldNot(HaveOccured())
		})

		Context("When the stop message is present", func() {
			It("can delete the stop message (and only cares about the relevant fields)", func() {
				toDelete := []models.QueueStopMessage{
					models.QueueStopMessage{InstanceGuid: message1.InstanceGuid},
					models.QueueStopMessage{InstanceGuid: message3.InstanceGuid},
				}
				err := store.DeleteQueueStopMessages(toDelete)
				Ω(err).ShouldNot(HaveOccured())

				desired, err := store.GetQueueStopMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(desired).Should(HaveLen(1))
				Ω(desired).Should(ContainElement(message2))
			})
		})

		Context("When the desired message key is not present", func() {
			It("returns an error, but does leave things in a broken state... for now...", func() {
				toDelete := []models.QueueStopMessage{
					models.QueueStopMessage{InstanceGuid: message1.InstanceGuid},
					models.QueueStopMessage{InstanceGuid: "floobedey"},
					models.QueueStopMessage{InstanceGuid: message3.InstanceGuid},
				}
				err := store.DeleteQueueStopMessages(toDelete)
				Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

				stop, err := store.GetQueueStopMessages()
				Ω(err).ShouldNot(HaveOccured())
				Ω(stop).Should(HaveLen(2))
				Ω(stop).Should(ContainElement(message2))
				Ω(stop).Should(ContainElement(message3))
			})
		})
	})
})
