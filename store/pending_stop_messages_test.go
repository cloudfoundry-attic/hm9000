package store_test

import (
	"time"

	"github.com/cloudfoundry/gunk/workpool"
	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter"
	"github.com/cloudfoundry/storeadapter/etcdstoreadapter"
	"github.com/cloudfoundry/storeadapter/storenodematchers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Storing PendingStopMessages", func() {
	var (
		store        Store
		storeAdapter storeadapter.StoreAdapter
		conf         *config.Config
		message1     models.PendingStopMessage
		message2     models.PendingStopMessage
		message3     models.PendingStopMessage
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Expect(err).NotTo(HaveOccurred())
		wpool, err := workpool.NewWorkPool(conf.StoreMaxConcurrentRequests)
		Expect(err).NotTo(HaveOccurred())
		storeAdapter, err = etcdstoreadapter.New(
			&etcdstoreadapter.ETCDOptions{ClusterUrls: etcdRunner.NodeURLS()},
			wpool,
		)
		Expect(err).NotTo(HaveOccurred())
		err = storeAdapter.Connect()
		Expect(err).NotTo(HaveOccurred())

		message1 = models.NewPendingStopMessage(time.Unix(100, 0), 10, 4, "ABC", "123", "XYZ", models.PendingStopMessageReasonInvalid)
		message2 = models.NewPendingStopMessage(time.Unix(100, 0), 10, 4, "DEF", "456", "ALPHA", models.PendingStopMessageReasonInvalid)
		message3 = models.NewPendingStopMessage(time.Unix(100, 0), 10, 4, "GHI", "789", "BETA", models.PendingStopMessageReasonInvalid)

		store = NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
	})

	AfterEach(func() {
		storeAdapter.Disconnect()
	})

	Describe("Saving stop messages", func() {
		BeforeEach(func() {
			err := store.SavePendingStopMessages(
				message1,
				message2,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("stores the passed in stop messages", func() {
			node, err := storeAdapter.ListRecursively("/hm/v1/stop")
			Expect(err).NotTo(HaveOccurred())
			Expect(node.ChildNodes).To(HaveLen(2))
			Expect(node.ChildNodes).To(ContainElement(storenodematchers.MatchStoreNode(storeadapter.StoreNode{
				Key:   "/hm/v1/stop/" + message1.StoreKey(),
				Value: message1.ToJSON(),
				TTL:   0,
			})))
			Expect(node.ChildNodes).To(ContainElement(storenodematchers.MatchStoreNode(storeadapter.StoreNode{
				Key:   "/hm/v1/stop/" + message2.StoreKey(),
				Value: message2.ToJSON(),
				TTL:   0,
			})))
		})
	})

	Describe("Fetching stop message", func() {
		Context("When the stop message is present", func() {
			BeforeEach(func() {
				err := store.SavePendingStopMessages(
					message1,
					message2,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("can fetch the stop message", func() {
				desired, err := store.GetPendingStopMessages()
				Expect(err).NotTo(HaveOccurred())
				Expect(desired).To(HaveLen(2))
				Expect(desired).To(ContainElement(message1))
				Expect(desired).To(ContainElement(message2))
			})
		})

		Context("when the stop message is empty", func() {
			BeforeEach(func() {
				hb := message1
				err := store.SavePendingStopMessages(hb)
				Expect(err).NotTo(HaveOccurred())
				err = store.DeletePendingStopMessages(hb)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an empty array", func() {
				stop, err := store.GetPendingStopMessages()
				Expect(err).NotTo(HaveOccurred())
				Expect(stop).To(BeEmpty())
			})
		})

		Context("When the stop message key is missing", func() {
			BeforeEach(func() {
				_, err := storeAdapter.ListRecursively("/hm/v1/stop")
				Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
			})

			It("returns an empty array and no error", func() {
				stop, err := store.GetPendingStopMessages()
				Expect(err).NotTo(HaveOccurred())
				Expect(stop).To(BeEmpty())
			})
		})
	})

	Describe("Deleting stop message", func() {
		BeforeEach(func() {
			err := store.SavePendingStopMessages(
				message1,
				message2,
				message3,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes stop messages (and only cares about the relevant fields)", func() {
			toDelete := []models.PendingStopMessage{
				models.NewPendingStopMessage(time.Time{}, 0, 0, "", "", message1.InstanceGuid, models.PendingStopMessageReasonInvalid),
				models.NewPendingStopMessage(time.Time{}, 0, 0, "", "", message3.InstanceGuid, models.PendingStopMessageReasonInvalid),
			}
			err := store.DeletePendingStopMessages(toDelete...)
			Expect(err).NotTo(HaveOccurred())

			desired, err := store.GetPendingStopMessages()
			Expect(err).NotTo(HaveOccurred())
			Expect(desired).To(HaveLen(1))
			Expect(desired).To(ContainElement(message2))
		})
	})
})
