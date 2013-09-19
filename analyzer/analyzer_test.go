package analyzer

// very much WIP

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
)

var _ = Describe("Analyzer", func() {
	var (
		analyzer  *Analyzer
		etcdStore store.Store
		conf      config.Config
		a1        app.App
		a2        app.App
	)

	BeforeEach(func() {
		var err error
		conf, err = config.DefaultConfig()
		Ω(err).ShouldNot(HaveOccured())

		etcdStore = store.NewETCDStore(etcdRunner.NodeURLS(), conf.StoreMaxConcurrentRequests)
		err = etcdStore.Connect()
		Ω(err).ShouldNot(HaveOccured())

		a1 = app.NewApp()
		a2 = app.NewApp()

		analyzer = New(etcdStore)
	})

	insertDesiredIntoStore := func(desired models.DesiredAppState) {
		key := "/desired/" + desired.StoreKey()
		value := desired.ToJson()

		node := store.StoreNode{
			Key:   key,
			Value: value,
		}

		etcdStore.Set([]store.StoreNode{node})
	}

	insertActualIntoStore := func(heartbeat models.InstanceHeartbeat) {
		key := "/actual/" + heartbeat.StoreKey()
		value := heartbeat.ToJson()

		node := store.StoreNode{
			Key:   key,
			Value: value,
		}
		etcdStore.Set([]store.StoreNode{node})
	}

	Context("When /desired and /actual are missing", func() {
		It("Should not send any start or stop messages", func() {
			startMessages, stopMessages, err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
			Ω(startMessages).Should(BeEmpty())
			Ω(stopMessages).Should(BeEmpty())
		})
	})

	Context("When /desired and /actual are empty", func() {
		BeforeEach(func() {
			desired := a1.DesiredState(42)
			actual := a2.GetInstance(0).Heartbeat(30)

			insertDesiredIntoStore(desired)
			insertActualIntoStore(actual)

			etcdStore.Delete("/desired/" + desired.StoreKey())
			etcdStore.Delete("/actual/" + actual.StoreKey())
		})

		It("Should not send any start or stop messages", func() {
			startMessages, stopMessages, err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
			Ω(startMessages).Should(BeEmpty())
			Ω(stopMessages).Should(BeEmpty())
		})
	})

	Context("where thare are desired instances and no running instances", func() {
		BeforeEach(func() {
			desired1 := a1.DesiredState(17)
			desired1.NumberOfInstances = 1
			desired2 := a2.DesiredState(34)
			desired2.NumberOfInstances = 3
			insertDesiredIntoStore(desired1)
			insertDesiredIntoStore(desired2)
		})

		It("Should return an array of start messages for the missing instances", func() {
			startMessages, stopMessages, err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
			Ω(stopMessages).Should(BeEmpty())
			Ω(startMessages).Should(HaveLen(2))
			Ω(startMessages).Should(ContainElement(models.QueueStartMessage{
				AppGuid:        a1.AppGuid,
				AppVersion:     a1.AppVersion,
				IndicesToStart: []int{0},
			}))
			Ω(startMessages).Should(ContainElement(models.QueueStartMessage{
				AppGuid:        a2.AppGuid,
				AppVersion:     a2.AppVersion,
				IndicesToStart: []int{0, 1, 2},
			}))
		})
	})

	Context("When there are actual instances and no desired instances", func() {
		BeforeEach(func() {
			insertActualIntoStore(a1.GetInstance(0).Heartbeat(12))
			insertActualIntoStore(a2.GetInstance(0).Heartbeat(17))
			insertActualIntoStore(a2.GetInstance(2).Heartbeat(1138))
		})

		It("Should return an array of stop messages for the extra instances", func() {
			startMessages, stopMessages, err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
			Ω(startMessages).Should(BeEmpty())
			Ω(stopMessages).Should(HaveLen(3))
			Ω(stopMessages).Should(ContainElement(models.QueueStopMessage{
				InstanceGuid: a1.GetInstance(0).InstanceGuid,
			}))
			Ω(stopMessages).Should(ContainElement(models.QueueStopMessage{
				InstanceGuid: a2.GetInstance(0).InstanceGuid,
			}))
			Ω(stopMessages).Should(ContainElement(models.QueueStopMessage{
				InstanceGuid: a2.GetInstance(2).InstanceGuid,
			}))
		})
	})

	Context("When there is one desired instance which is running", func() {
		BeforeEach(func() {
			desired := a1.DesiredState(10)
			desired.NumberOfInstances = 1
			insertDesiredIntoStore(desired)
			insertActualIntoStore(a1.GetInstance(0).Heartbeat(12))
		})

		It("Should not send any start or stop messages", func() {
			startMessages, stopMessages, err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
			Ω(startMessages).Should(BeEmpty())
			Ω(stopMessages).Should(BeEmpty())
		})
	})
})
