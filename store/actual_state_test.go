package store_test

import (
	. "github.com/cloudfoundry/hm9000/store"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/storeadapter"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
)

var _ = Describe("Actual State", func() {
	var (
		store       Store
		etcdAdapter storeadapter.StoreAdapter
		conf        config.Config
		heartbeat1  models.InstanceHeartbeat
		heartbeat2  models.InstanceHeartbeat
		heartbeat3  models.InstanceHeartbeat
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())
		etcdAdapter = storeadapter.NewETCDStoreAdapter(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err = etcdAdapter.Connect()
		Ω(err).ShouldNot(HaveOccured())

		a := app.NewApp()
		heartbeat1 = a.GetInstance(0).Heartbeat(17)
		heartbeat2 = a.GetInstance(1).Heartbeat(12)
		heartbeat3 = a.GetInstance(2).Heartbeat(14)

		store = NewStore(conf, etcdAdapter, fakelogger.NewFakeLogger())
	})

	AfterEach(func() {
		etcdAdapter.Disconnect()
	})

	Describe("Saving actual state ", func() {
		BeforeEach(func() {
			err := store.SaveActualState([]models.InstanceHeartbeat{
				heartbeat1,
				heartbeat2,
			})
			Ω(err).ShouldNot(HaveOccured())
		})

		It("can stores the passed in actual state", func() {
			nodes, err := etcdAdapter.List("/actual")
			Ω(err).ShouldNot(HaveOccured())
			Ω(nodes).Should(HaveLen(2))
			Ω(nodes).Should(ContainElement(storeadapter.StoreNode{
				Key:   "/actual/" + heartbeat1.InstanceGuid,
				Value: heartbeat1.ToJSON(),
				TTL:   conf.HeartbeatTTL() - 1,
			}))
			Ω(nodes).Should(ContainElement(storeadapter.StoreNode{
				Key:   "/actual/" + heartbeat2.InstanceGuid,
				Value: heartbeat2.ToJSON(),
				TTL:   conf.HeartbeatTTL() - 1,
			}))
		})
	})

	Describe("Fetching actual state", func() {
		Context("When the actual state is present", func() {
			BeforeEach(func() {
				err := store.SaveActualState([]models.InstanceHeartbeat{
					heartbeat1,
					heartbeat2,
				})
				Ω(err).ShouldNot(HaveOccured())
			})

			It("can fetch the actual state", func() {
				desired, err := store.GetActualState()
				Ω(err).ShouldNot(HaveOccured())
				Ω(desired).Should(HaveLen(2))
				Ω(desired).Should(ContainElement(heartbeat1))
				Ω(desired).Should(ContainElement(heartbeat2))
			})
		})

		Context("when the actual state is empty", func() {
			BeforeEach(func() {
				hb := heartbeat1
				err := store.SaveActualState([]models.InstanceHeartbeat{hb})
				Ω(err).ShouldNot(HaveOccured())
				err = store.DeleteActualState([]models.InstanceHeartbeat{hb})
				Ω(err).ShouldNot(HaveOccured())
			})

			It("returns an empty array", func() {
				actual, err := store.GetActualState()
				Ω(err).ShouldNot(HaveOccured())
				Ω(actual).Should(BeEmpty())
			})
		})

		Context("When the actual state key is missing", func() {
			BeforeEach(func() {
				_, err := etcdAdapter.List("/actual")
				Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))
			})

			It("returns an empty array and no error", func() {
				actual, err := store.GetActualState()
				Ω(err).ShouldNot(HaveOccured())
				Ω(actual).Should(BeEmpty())
			})
		})
	})

	Describe("Deleting actual state", func() {
		BeforeEach(func() {
			err := store.SaveActualState([]models.InstanceHeartbeat{
				heartbeat1,
				heartbeat2,
				heartbeat3,
			})
			Ω(err).ShouldNot(HaveOccured())
		})

		Context("When the actual state is present", func() {
			It("can delete the actual state (and only cares about the relevant fields)", func() {
				toDelete := []models.InstanceHeartbeat{
					models.InstanceHeartbeat{InstanceGuid: heartbeat1.InstanceGuid},
					models.InstanceHeartbeat{InstanceGuid: heartbeat3.InstanceGuid},
				}
				err := store.DeleteActualState(toDelete)
				Ω(err).ShouldNot(HaveOccured())

				desired, err := store.GetActualState()
				Ω(err).ShouldNot(HaveOccured())
				Ω(desired).Should(HaveLen(1))
				Ω(desired).Should(ContainElement(heartbeat2))
			})
		})

		Context("When the desired state key is not present", func() {
			It("returns an error, but does leave things in a broken state... for now...", func() {
				toDelete := []models.InstanceHeartbeat{
					models.InstanceHeartbeat{InstanceGuid: heartbeat1.InstanceGuid},
					models.InstanceHeartbeat{InstanceGuid: "floobedey"},
					models.InstanceHeartbeat{InstanceGuid: heartbeat3.InstanceGuid},
				}
				err := store.DeleteActualState(toDelete)
				Ω(err).Should(Equal(storeadapter.ErrorKeyNotFound))

				actual, err := store.GetActualState()
				Ω(err).ShouldNot(HaveOccured())
				Ω(actual).Should(HaveLen(2))
				Ω(actual).Should(ContainElement(heartbeat2))
				Ω(actual).Should(ContainElement(heartbeat3))
			})
		})
	})
})
