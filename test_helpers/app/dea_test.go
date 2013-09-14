package app

import (
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dea", func() {
	var dea Dea

	BeforeEach(func() {
		dea = NewDea()
	})

	It("should have a GUID", func() {
		Ω(dea.DeaGuid).ShouldNot(BeZero())
	})

	Describe("Generating apps", func() {
		It("memoizes the app", func() {
			Ω(dea.GetApp(0)).Should(Equal(dea.GetApp(0)))
		})

		It("assigns the app's DeaGuid", func() {
			Ω(dea.GetApp(0).Heartbeat(1, 17).DeaGuid).Should(Equal(dea.DeaGuid))
		})
	})

	Describe("heartbeat", func() {
		var heartbeat models.Heartbeat

		BeforeEach(func() {
			heartbeat = dea.Heartbeat(70, 17)
		})

		It("returns a heartbeat with the requested number of apps, each app having one instance", func() {
			Ω(heartbeat.DeaGuid).Should(Equal(dea.DeaGuid))
			Ω(heartbeat.InstanceHeartbeats).Should(HaveLen(70))
			Ω(heartbeat.InstanceHeartbeats[0].StateTimestamp).Should(Equal(17.0))
			Ω(heartbeat.InstanceHeartbeats[0].AppGuid).ShouldNot(Equal(heartbeat.InstanceHeartbeats[1].AppGuid))
		})

		Context("requesting the heartbeat again", func() {
			It("returns the same heartbeat", func() {
				Ω(dea.Heartbeat(70, 17)).Should(Equal(heartbeat))
			})
		})
	})
})
