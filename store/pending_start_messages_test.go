package store_test

import (
	"time"

	"code.cloudfoundry.org/workpool"
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

var _ = Describe("Storing PendingStartMessages", func() {
	var (
		store        Store
		storeAdapter storeadapter.StoreAdapter
		conf         *config.Config
		message1     models.PendingStartMessage
		message2     models.PendingStartMessage
		message3     models.PendingStartMessage
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

		message1 = models.NewPendingStartMessage(time.Unix(100, 0), 10, 4, "ABC", "123", 1, 1.0, models.PendingStartMessageReasonInvalid)
		message2 = models.NewPendingStartMessage(time.Unix(100, 0), 10, 4, "DEF", "123", 1, 1.0, models.PendingStartMessageReasonInvalid)
		message3 = models.NewPendingStartMessage(time.Unix(100, 0), 10, 4, "ABC", "456", 1, 1.0, models.PendingStartMessageReasonInvalid)

		store = NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
	})

	AfterEach(func() {
		storeAdapter.Disconnect()
	})

	Describe("Saving start messages", func() {
		BeforeEach(func() {
			err := store.SavePendingStartMessages(
				message1,
				message2,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("stores the passed in start messages", func() {
			node, err := storeAdapter.ListRecursively("/hm/v1/start")
			Expect(err).NotTo(HaveOccurred())
			Expect(node.ChildNodes).To(HaveLen(2))
			Expect(node.ChildNodes).To(ContainElement(storenodematchers.MatchStoreNode(storeadapter.StoreNode{
				Key:   "/hm/v1/start/" + message1.StoreKey(),
				Value: message1.ToJSON(),
				TTL:   0,
			})))
			Expect(node.ChildNodes).To(ContainElement(storenodematchers.MatchStoreNode(storeadapter.StoreNode{
				Key:   "/hm/v1/start/" + message2.StoreKey(),
				Value: message2.ToJSON(),
				TTL:   0,
			})))
		})
	})

	Describe("Fetching start message", func() {
		Context("When the start message is present", func() {
			BeforeEach(func() {
				err := store.SavePendingStartMessages(
					message1,
					message2,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("can fetch the start message", func() {
				desired, err := store.GetPendingStartMessages()
				Expect(err).NotTo(HaveOccurred())
				Expect(desired).To(HaveLen(2))
				Expect(desired).To(ContainElement(message1))
				Expect(desired).To(ContainElement(message2))
			})
		})

		Context("when the start message is empty", func() {
			BeforeEach(func() {
				hb := message1
				err := store.SavePendingStartMessages(hb)
				Expect(err).NotTo(HaveOccurred())
				err = store.DeletePendingStartMessages(hb)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an empty array", func() {
				start, err := store.GetPendingStartMessages()
				Expect(err).NotTo(HaveOccurred())
				Expect(start).To(BeEmpty())
			})
		})

		Context("When the start message key is missing", func() {
			BeforeEach(func() {
				_, err := storeAdapter.ListRecursively("/hm/v1/start")
				Expect(err).To(Equal(storeadapter.ErrorKeyNotFound))
			})

			It("returns an empty array and no error", func() {
				start, err := store.GetPendingStartMessages()
				Expect(err).NotTo(HaveOccurred())
				Expect(start).To(BeEmpty())
			})
		})
	})

	Describe("Deleting start message", func() {
		BeforeEach(func() {
			err := store.SavePendingStartMessages(
				message1,
				message2,
				message3,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("can deletes start messages", func() {
			toDelete := []models.PendingStartMessage{
				models.NewPendingStartMessage(time.Time{}, 0, 0, message1.AppGuid, message1.AppVersion, message1.IndexToStart, 0, models.PendingStartMessageReasonInvalid),
				models.NewPendingStartMessage(time.Time{}, 0, 0, message3.AppGuid, message3.AppVersion, message3.IndexToStart, 0, models.PendingStartMessageReasonInvalid),
			}
			err := store.DeletePendingStartMessages(toDelete...)
			Expect(err).NotTo(HaveOccurred())

			desired, err := store.GetPendingStartMessages()
			Expect(err).NotTo(HaveOccurred())
			Expect(desired).To(HaveLen(1))
			Expect(desired).To(ContainElement(message2))
		})
	})
})
