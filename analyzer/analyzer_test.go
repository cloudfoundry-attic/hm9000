package analyzer_test

import (
	. "github.com/cloudfoundry/hm9000/analyzer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/app"
	"github.com/cloudfoundry/hm9000/testhelpers/fakeoutbox"
	"github.com/cloudfoundry/hm9000/testhelpers/fakestore"

	"errors"
)

var _ = Describe("Analyzer", func() {
	var (
		analyzer *Analyzer
		store    *fakestore.FakeStore
		outbox   *fakeoutbox.FakeOutbox
		a        app.App
	)

	BeforeEach(func() {
		store = fakestore.NewFakeStore()
		outbox = fakeoutbox.NewFakeOutbox()

		a = app.NewApp()

		analyzer = New(store, outbox)
	})

	Describe("Handling store errors", func() {
		Context("When fetching desired state fails with an error", func() {
			BeforeEach(func() {
				store.GetDesiredStateError = errors.New("oops!")
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).Should(Equal(errors.New("oops!")))
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})

		Context("When fetching actual state fails with an error", func() {
			BeforeEach(func() {
				store.GetActualStateError = errors.New("oops!")
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).Should(Equal(errors.New("oops!")))
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})
	})

	// Describe("Handling store freshness", func() {
	// 	BeforeEach(func() {
	// 		desired := a.DesiredState(0)
	// 		store.SaveDesiredState([]models.DesiredAppState{
	// 			desired,
	// 		})
	// 		store.SaveActualState([]models.InstanceHeartbeat{
	// 			app.NewApp().GetInstance(0).Heartbeat(0),
	// 		})
	// 	})

	// 	Context("when the desired state is not fresh", func() {
	// 		It("should not send any start or stop messages", func() {
	// 			err := analyzer.Analyze()
	// 			Ω(err).ShouldNot(HaveOccured())
	// 			Ω(outbox.StartMessages).Should(BeEmpty())
	// 			Ω(outbox.StopMessages).Should(BeEmpty())
	// 		})
	// 	})

	// 	Context("when the actual state is not fresh", func() {
	// 		It("should not send any start or stop messages", func() {
	// 			err := analyzer.Analyze()
	// 			Ω(err).ShouldNot(HaveOccured())
	// 			Ω(outbox.StartMessages).Should(BeEmpty())
	// 			Ω(outbox.StopMessages).Should(BeEmpty())
	// 		})
	// 	})
	// })

	Describe("The steady state", func() {
		Context("When there are no desired or running apps", func() {
			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})

		Context("When the desired number of instances and the running number of instances match", func() {
			BeforeEach(func() {
				desired := a.DesiredState(0)
				desired.State = models.AppStateStarted
				desired.NumberOfInstances = 3
				store.SaveDesiredState([]models.DesiredAppState{
					desired,
				})
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(0).Heartbeat(0),
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(2).Heartbeat(0),
				})
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})

		Context("When there are stopped apps and no running instances for that app", func() {
			BeforeEach(func() {
				desired := a.DesiredState(10)
				desired.State = models.AppStateStopped
				desired.NumberOfInstances = 3
				store.SaveDesiredState([]models.DesiredAppState{
					desired,
				})
			})

			It("should not send any start or stop messages", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(BeEmpty())
			})
		})
	})

	Describe("Starting missing instances", func() {
		Context("where an app has desired instances", func() {
			BeforeEach(func() {
				desired := a.DesiredState(0)
				desired.NumberOfInstances = 4
				store.SaveDesiredState([]models.DesiredAppState{
					desired,
				})
			})

			Context("and none of the instances are running", func() {
				It("should return a start message for the missing instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.StopMessages).Should(BeEmpty())
					Ω(outbox.StartMessages).Should(HaveLen(1))
					Ω(outbox.StartMessages[0]).Should(Equal(models.QueueStartMessage{
						AppGuid:        a.AppGuid,
						AppVersion:     a.AppVersion,
						IndicesToStart: []int{0, 1, 2, 3},
					}))
				})
			})

			Context("but only some of the instances are running", func() {
				BeforeEach(func() {
					store.SaveActualState([]models.InstanceHeartbeat{
						a.GetInstance(0).Heartbeat(0),
						a.GetInstance(2).Heartbeat(0),
					})
				})

				It("should return a start message containing only the missing indices", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.StopMessages).Should(BeEmpty())
					Ω(outbox.StartMessages).Should(HaveLen(1))
					Ω(outbox.StartMessages[0]).Should(Equal(models.QueueStartMessage{
						AppGuid:        a.AppGuid,
						AppVersion:     a.AppVersion,
						IndicesToStart: []int{1, 3},
					}))
				})
			})
		})
	})

	Describe("Stopping extra instances", func() {
		Context("When there are running instances", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(0).Heartbeat(0),
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(2).Heartbeat(0),
				})
			})

			Context("but no desired instances", func() {
				It("should return an array of stop messages for the extra instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.StartMessages).Should(BeEmpty())
					Ω(outbox.StopMessages).Should(HaveLen(3))
					Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
						InstanceGuid: a.GetInstance(0).InstanceGuid,
					}))
					Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
						InstanceGuid: a.GetInstance(1).InstanceGuid,
					}))
					Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
						InstanceGuid: a.GetInstance(2).InstanceGuid,
					}))
				})
			})

			Context("and the desired app is in the STOPPED state", func() {
				BeforeEach(func() {
					desired := a.DesiredState(0)
					desired.NumberOfInstances = 3
					desired.State = models.AppStateStopped
					store.SaveDesiredState([]models.DesiredAppState{
						desired,
					})
				})

				It("should return an array of stop messages for the extra instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.StartMessages).Should(BeEmpty())
					Ω(outbox.StopMessages).Should(HaveLen(3))
					Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
						InstanceGuid: a.GetInstance(0).InstanceGuid,
					}))
					Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
						InstanceGuid: a.GetInstance(1).InstanceGuid,
					}))
					Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
						InstanceGuid: a.GetInstance(2).InstanceGuid,
					}))
				})
			})

			Context("and the desired app desires fewer instances", func() {
				BeforeEach(func() {
					desired := a.DesiredState(0)
					desired.NumberOfInstances = 1
					store.SaveDesiredState([]models.DesiredAppState{
						desired,
					})
				})

				It("should return an array of stop messages for the (correct) extra instances", func() {
					err := analyzer.Analyze()
					Ω(err).ShouldNot(HaveOccured())
					Ω(outbox.StartMessages).Should(BeEmpty())
					Ω(outbox.StopMessages).Should(HaveLen(2))
					Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
						InstanceGuid: a.GetInstance(1).InstanceGuid,
					}))
					Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
						InstanceGuid: a.GetInstance(2).InstanceGuid,
					}))
				})
			})

		})
	})

	Describe("Interesting edge cases involving index-mismatches", func() {
		BeforeEach(func() {
			desired := a.DesiredState(0)
			desired.NumberOfInstances = 3
			store.SaveDesiredState([]models.DesiredAppState{
				desired,
			})
		})

		Context("when *enough* apps are running, but there are an indices missing", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(3).Heartbeat(0),
					a.GetInstance(4).Heartbeat(0),
				})
			})

			It("should return a start message containing the missing indices and *no* stop message", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StopMessages).Should(BeEmpty())
				Ω(outbox.StartMessages).Should(HaveLen(1))
				Ω(outbox.StartMessages[0]).Should(Equal(models.QueueStartMessage{
					AppGuid:        a.AppGuid,
					AppVersion:     a.AppVersion,
					IndicesToStart: []int{0, 2},
				}))
			})
		})

		Context("when more than *enough* apps are running, but there are indices missing", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(3).Heartbeat(0),
					a.GetInstance(4).Heartbeat(0),
					a.GetInstance(5).Heartbeat(0),
					a.GetInstance(6).Heartbeat(0),
				})
			})

			It("should return a start message containing the missing indices and a stop message for the extra instances", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StartMessages).Should(HaveLen(1))
				Ω(outbox.StartMessages[0]).Should(Equal(models.QueueStartMessage{
					AppGuid:        a.AppGuid,
					AppVersion:     a.AppVersion,
					IndicesToStart: []int{0, 2},
				}))
				Ω(outbox.StopMessages).Should(HaveLen(2))
				Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
					InstanceGuid: a.GetInstance(5).InstanceGuid,
				}))
				Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
					InstanceGuid: a.GetInstance(6).InstanceGuid,
				}))
			})
		})

		Context("when the missing indices start", func() {
			BeforeEach(func() {
				store.SaveActualState([]models.InstanceHeartbeat{
					a.GetInstance(0).Heartbeat(0),
					a.GetInstance(1).Heartbeat(0),
					a.GetInstance(2).Heartbeat(0),
					a.GetInstance(3).Heartbeat(0),
					a.GetInstance(4).Heartbeat(0),
				})
			})

			It("should stop the extra indices", func() {
				err := analyzer.Analyze()
				Ω(err).ShouldNot(HaveOccured())
				Ω(outbox.StartMessages).Should(BeEmpty())
				Ω(outbox.StopMessages).Should(HaveLen(2))
				Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
					InstanceGuid: a.GetInstance(3).InstanceGuid,
				}))
				Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
					InstanceGuid: a.GetInstance(4).InstanceGuid,
				}))
			})
		})

	})

	Describe("Processing multiple apps", func() {
		var (
			otherApp app.App
			olderApp app.App
		)

		BeforeEach(func() {
			otherApp = app.NewApp()
			olderApp = app.NewApp()
			olderApp.AppGuid = a.AppGuid

			olderDesired := olderApp.DesiredState(0)
			olderDesired.State = models.AppStateStopped

			otherDesired := otherApp.DesiredState(0)
			otherDesired.NumberOfInstances = 3

			store.SaveDesiredState([]models.DesiredAppState{
				a.DesiredState(0),
				otherDesired,
				olderDesired,
			})
			store.SaveActualState([]models.InstanceHeartbeat{
				a.GetInstance(0).Heartbeat(0),
				a.GetInstance(1).Heartbeat(0),
				olderApp.GetInstance(0).Heartbeat(0),
				otherApp.GetInstance(0).Heartbeat(0),
				otherApp.GetInstance(2).Heartbeat(0),
			})
		})

		It("should analyze each app-version combination separately", func() {
			err := analyzer.Analyze()
			Ω(err).ShouldNot(HaveOccured())
			Ω(outbox.StartMessages).Should(HaveLen(1))
			Ω(outbox.StopMessages).Should(HaveLen(2))
			Ω(outbox.StartMessages).Should(ContainElement(models.QueueStartMessage{
				AppGuid:        otherApp.AppGuid,
				AppVersion:     otherApp.AppVersion,
				IndicesToStart: []int{1},
			}))
			Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
				InstanceGuid: a.GetInstance(1).InstanceGuid,
			}))
			Ω(outbox.StopMessages).Should(ContainElement(models.QueueStopMessage{
				InstanceGuid: olderApp.GetInstance(0).InstanceGuid,
			}))
		})
	})
})
