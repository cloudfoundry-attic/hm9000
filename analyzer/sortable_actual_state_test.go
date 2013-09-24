package analyzer_test

import (
	. "github.com/cloudfoundry/hm9000/analyzer"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SortableActualState", func() {
	var (
		sortable SortableActualState
		a        app.App
	)

	BeforeEach(func() {
		a = app.NewApp()
		sortable = SortableActualState{
			a.GetInstance(0).Heartbeat(0),
			a.GetInstance(3).Heartbeat(0),
			a.GetInstance(1).Heartbeat(0),
			a.GetInstance(2).Heartbeat(0),
		}
	})

	Describe("The sort interface", func() {
		Describe("Len()", func() {
			It("returns the length of the array", func() {
				Ω(sortable.Len()).Should(Equal(4))
			})
		})

		Describe("Less()", func() {
			It("compares the InstanceIndex of the heartbeats at the passed in indices", func() {
				Ω(sortable.Less(0, 1)).Should(BeTrue())
				Ω(sortable.Less(1, 0)).Should(BeFalse())
				Ω(sortable.Less(1, 3)).Should(BeFalse())
				Ω(sortable.Less(3, 1)).Should(BeTrue())
			})
		})

		Describe("Swap()", func() {
			It("should swap the passed in indices", func() {
				sortable.Swap(0, 2)
				Ω(sortable[0]).Should(Equal(a.GetInstance(1).Heartbeat(0)))
				Ω(sortable[2]).Should(Equal(a.GetInstance(0).Heartbeat(0)))
			})
		})
	})

	Describe("SortDescendingInPlace", func() {
		It("should sort the array, in place, in descending order by index", func() {
			sortable.SortDescendingInPlace()
			Ω(sortable).Should(Equal(SortableActualState{
				a.GetInstance(3).Heartbeat(0),
				a.GetInstance(2).Heartbeat(0),
				a.GetInstance(1).Heartbeat(0),
				a.GetInstance(0).Heartbeat(0),
			}))
		})
	})
})
