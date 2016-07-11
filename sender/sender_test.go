package sender_test

import (
	"errors"
	"strings"
	"time"

	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/cloudfoundry/hm9000/config"
	"github.com/cloudfoundry/hm9000/helpers/metricsaccountant/fakemetricsaccountant"
	"github.com/cloudfoundry/hm9000/models"
	. "github.com/cloudfoundry/hm9000/sender"
	storepackage "github.com/cloudfoundry/hm9000/store"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	"github.com/cloudfoundry/hm9000/testhelpers/fakelogger"
	"github.com/cloudfoundry/storeadapter/fakestoreadapter"
	"github.com/cloudfoundry/yagnats/fakeyagnats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"code.cloudfoundry.org/clock/fakeclock"
)

var _ = Describe("Sender", func() {
	var (
		storeAdapter          *fakestoreadapter.FakeStoreAdapter
		store                 storepackage.Store
		sender                Sender
		messageBus            *fakeyagnats.FakeNATSConn
		timeProvider          *fakeclock.FakeClock
		logger                *fakelogger.FakeLogger
		dea                   appfixture.DeaFixture
		app                   appfixture.AppFixture
		conf                  *config.Config
		metricsAccountant     *fakemetricsaccountant.FakeMetricsAccountant
		apps                  map[string]*models.App
		startMessages         []models.PendingStartMessage
		stopMessages          []models.PendingStopMessage
		receivedStartMessages []models.StartMessage
		receivedStopMessages  []models.StopMessage
		server                *httptest.Server
		httpError             error
		receivedEndpoint      string
		receivedAuth          string
	)

	BeforeEach(func() {
		messageBus = fakeyagnats.Connect()
		dea = appfixture.NewDeaFixture()
		app = dea.GetApp(0)
		conf, _ = config.DefaultConfig()
		metricsAccountant = &fakemetricsaccountant.FakeMetricsAccountant{}

		timeProvider = fakeclock.NewFakeClock(time.Unix(int64(10+conf.ActualFreshnessTTL()), 0))

		storeAdapter = fakestoreadapter.New()
		store = storepackage.NewStore(conf, storeAdapter, fakelogger.NewFakeLogger())
		logger = fakelogger.NewFakeLogger()
		sender = New(store, metricsAccountant, conf, messageBus, logger, timeProvider)
		store.BumpActualFreshness(time.Unix(10, 0))
		apps = make(map[string]*models.App)

		startMessages = []models.PendingStartMessage{}
		stopMessages = []models.PendingStopMessage{}

		receivedStartMessages = []models.StartMessage{}
		receivedStopMessages = []models.StopMessage{}

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := ioutil.ReadAll(r.Body)
			r.Body.Close()
			Expect(err).ToNot(HaveOccurred())

			var stopMessage models.StopMessage
			var startMessage models.StartMessage
			if strings.Contains((string)(b), "is_duplicate") {
				stopMessage, httpError = models.NewStopMessageFromJSON(b)
				Expect(err).ToNot(HaveOccurred())
				receivedStopMessages = append(receivedStopMessages, stopMessage)
			} else {
				startMessage, httpError = models.NewStartMessageFromJSON(b)
				Expect(err).ToNot(HaveOccurred())
				receivedStartMessages = append(receivedStartMessages, startMessage)
			}

			receivedEndpoint = r.URL.String()
			receivedAuth = r.Header.Get("Authorization")
			w.WriteHeader(200)
		}))
		conf.CCInternalURL = server.URL
	})

	AfterEach(func() {
		server.Close()
	})

	Context("when there are no start messages in the queue", func() {
		It("should not send any messages", func() {
			err := sender.Send(timeProvider, apps, startMessages, stopMessages)
			Expect(err).NotTo(HaveOccurred())
			Expect(messageBus.PublishedMessageCount()).To(Equal(0))
		})
	})

	Context("when there are no stop messages in the queue", func() {
		It("should not send any messages", func() {
			err := sender.Send(timeProvider, apps, startMessages, stopMessages)
			Expect(err).NotTo(HaveOccurred())
			Expect(messageBus.PublishedMessageCount()).To(Equal(0))
			Expect(receivedStopMessages).To(HaveLen(0))
		})
	})

	Context("when there are start messages", func() {
		var keepAliveTime int
		var sentOn int64
		var err error
		var pendingMessage models.PendingStartMessage
		var storeSetErrInjector *fakestoreadapter.FakeStoreAdapterErrorInjector

		JustBeforeEach(func() {
			desired := app.DesiredState(1)
			apps[desired.StoreKey()] = models.NewApp(app.AppGuid, app.AppVersion, desired, nil, nil)

			pendingMessage = models.NewPendingStartMessage(time.Unix(100, 0), 30, keepAliveTime, app.AppGuid, app.AppVersion, 0, 1.0, models.PendingStartMessageReasonInvalid)
			pendingMessage.SentOn = sentOn

			store.SavePendingStartMessages(
				pendingMessage,
			)
			startMessages = append(startMessages, pendingMessage)

			storeAdapter.SetErrInjector = storeSetErrInjector
			err = sender.Send(timeProvider, apps, startMessages, stopMessages)
		})

		BeforeEach(func() {
			keepAliveTime = 0
			sentOn = 0
			err = nil
			storeSetErrInjector = nil
		})

		Context("and it is not time to send the message yet", func() {
			BeforeEach(func() {
				timeProvider = fakeclock.NewFakeClock(time.Unix(129, 0))
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not send the messages", func() {
				Expect(messageBus.PublishedMessages("hm9000.start")).To(HaveLen(0))
			})

			It("should not increment the metrics", func() {
				Expect(pendingStartMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(HaveLen(0))
			})

			It("should leave the messages in the queue", func() {
				messages, _ := store.GetPendingStartMessages()
				Expect(messages).To(HaveLen(1))
			})
		})

		Context("and it is time to send the message", func() {
			BeforeEach(func() {
				timeProvider = fakeclock.NewFakeClock(time.Unix(130, 0))
			})

			It("should send the message", func() {
				Expect(receivedStartMessages).To(HaveLen(1))
				Expect(receivedStartMessages[0]).To(Equal(models.StartMessage{
					AppGuid:       app.AppGuid,
					AppVersion:    app.AppVersion,
					InstanceIndex: 0,
					MessageId:     pendingMessage.MessageId,
				}))
			})

			It("should increment the metrics for that message", func() {
				Expect(pendingStartMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(ContainElement(pendingMessage))
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the message should be kept alive", func() {
				BeforeEach(func() {
					keepAliveTime = 30
				})

				It("should update the sent on times", func() {
					messages, _ := store.GetPendingStartMessages()
					Expect(messages).To(HaveLen(1))
					for _, message := range messages {
						Expect(message.SentOn).To(Equal(timeProvider.Now().Unix()))
					}
				})

				Context("when saving the start messages fails", func() {
					BeforeEach(func() {
						storeSetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("start", errors.New("oops"))
					})

					It("should return an error", func() {
						Expect(err).To(HaveOccurred())
					})
				})
			})

			Context("when the KeepAlive = 0", func() {
				BeforeEach(func() {
					keepAliveTime = 0
				})

				It("should just delete the message after sending it", func() {
					messages, _ := store.GetPendingStartMessages()
					Expect(messages).To(BeEmpty())
				})

				Context("when deleting the start messages fails", func() {
					BeforeEach(func() {
						storeAdapter.DeleteErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("start", errors.New("oops"))
					})

					It("should return an error", func() {
						Expect(err).To(HaveOccurred())
					})
				})
			})

			Context("when the message fails to send", func() {
				BeforeEach(func() {
					server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						b, err := ioutil.ReadAll(r.Body)
						r.Body.Close()
						Expect(err).ToNot(HaveOccurred())

						startMessage, err := models.NewStartMessageFromJSON(b)
						Expect(err).ToNot(HaveOccurred())

						receivedStartMessages = append(receivedStartMessages, startMessage)

						w.WriteHeader(500)
						fmt.Fprintln(w, "Throwing a 500")
					}))

					conf.CCInternalURL = server.URL
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
				})

				It("should not increment the metrics", func() {
					Expect(pendingStartMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(HaveLen(0))
				})
			})
		})

		Context("When the message has already been sent", func() {
			BeforeEach(func() {
				sentOn = 130
				keepAliveTime = 30
			})

			It("should not increment the metrics", func() {
				Expect(pendingStartMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(HaveLen(0))
			})

			Context("and the keep alive has elapsed", func() {
				BeforeEach(func() {
					timeProvider = fakeclock.NewFakeClock(time.Unix(160, 0))
				})

				It("should delete the message and not send it", func() {
					messages, _ := store.GetPendingStartMessages()
					Expect(messages).To(BeEmpty())
					Expect(messageBus.PublishedMessages("hm9000.start")).To(HaveLen(0))
				})
			})

			Context("and the keep alive has not elapsed", func() {
				BeforeEach(func() {
					timeProvider = fakeclock.NewFakeClock(time.Unix(159, 0))
				})

				It("should neither delete the message nor send it", func() {
					messages, _ := store.GetPendingStartMessages()
					Expect(messages).To(HaveLen(1))
					Expect(messageBus.PublishedMessages("hm9000.start")).To(HaveLen(0))
				})
			})
		})
	})

	Context("when there are stop messages", func() {
		var keepAliveTime int
		var sentOn int64
		var err error
		var pendingMessage models.PendingStopMessage
		var storeSetErrInjector *fakestoreadapter.FakeStoreAdapterErrorInjector

		JustBeforeEach(func() {
			heartbeat := app.Heartbeat(2)
			desired := app.DesiredState(0)
			apps[desired.StoreKey()] = models.NewApp(app.AppGuid, app.AppVersion, desired, heartbeat.InstanceHeartbeats, nil)

			pendingMessage = models.NewPendingStopMessage(time.Unix(100, 0), 30, keepAliveTime, app.AppGuid, app.AppVersion, app.InstanceAtIndex(0).InstanceGuid, models.PendingStopMessageReasonInvalid)
			pendingMessage.SentOn = sentOn
			store.SavePendingStopMessages(
				pendingMessage,
			)
			stopMessages = append(stopMessages, pendingMessage)

			storeAdapter.SetErrInjector = storeSetErrInjector
			err = sender.Send(timeProvider, apps, startMessages, stopMessages)
		})

		BeforeEach(func() {
			keepAliveTime = 0
			sentOn = 0
			err = nil
			storeSetErrInjector = nil
		})

		Context("and it is not time to send the message yet", func() {
			BeforeEach(func() {
				timeProvider = fakeclock.NewFakeClock(time.Unix(129, 0))
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not send the messages", func() {
				Expect(receivedStopMessages).To(HaveLen(0))
			})

			It("should leave the messages in the queue", func() {
				messages, _ := store.GetPendingStopMessages()
				Expect(messages).To(HaveLen(1))
			})

			It("should not increment the metrics", func() {
				Expect(pendingStartMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(HaveLen(0))
			})
		})

		Context("and it is time to send the message", func() {
			BeforeEach(func() {
				timeProvider = fakeclock.NewFakeClock(time.Unix(130, 0))
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should send the message", func() {
				Expect(receivedStopMessages).To(HaveLen(1))
				Expect(httpError).NotTo(HaveOccurred())

				expectedEndpoint := fmt.Sprintf("/internal/dea/hm9000/stop/%s", app.AppGuid)
				Expect(receivedEndpoint).To(Equal(expectedEndpoint))
			})

			It("uses the correct authorization", func() {
				expectedAuth := models.BasicAuthInfo{
					User:     "magnet",
					Password: "orangutan4sale",
				}.Encode()

				Expect(receivedAuth).To(Equal(expectedAuth))
			})

			It("Stops the correct app", func() {
				Expect(receivedStopMessages).To(HaveLen(1))
				Expect(receivedStopMessages[0].AppGuid).To(Equal(app.AppGuid))
			})

			It("should increment the metrics", func() {
				Expect(pendingStopMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(ContainElement(pendingMessage))
			})

			Context("when the message should be kept alive", func() {
				BeforeEach(func() {
					keepAliveTime = 30
				})

				It("should update the sent on times", func() {
					messages, _ := store.GetPendingStopMessages()
					Expect(messages).To(HaveLen(1))
					for _, message := range messages {
						Expect(message.SentOn).To(Equal(timeProvider.Now().Unix()))
					}
				})

				Context("when saving the stop message fails", func() {
					BeforeEach(func() {
						storeSetErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("stop", errors.New("oops"))
					})

					It("should return an error", func() {
						Expect(err).To(HaveOccurred())
					})
				})
			})

			Context("when the KeepAlive = 0", func() {
				BeforeEach(func() {
					keepAliveTime = 0
				})

				It("should just delete the message after sending it", func() {
					messages, _ := store.GetPendingStopMessages()
					Expect(messages).To(BeEmpty())
				})

				Context("when deleting the message fails", func() {
					BeforeEach(func() {
						storeAdapter.DeleteErrInjector = fakestoreadapter.NewFakeStoreAdapterErrorInjector("stop", errors.New("oops"))
					})

					It("should return an error", func() {
						Expect(err).To(HaveOccurred())
					})
				})
			})

			Context("when the message fails to send", func() {
				BeforeEach(func() {
					server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						b, err := ioutil.ReadAll(r.Body)
						r.Body.Close()
						Expect(err).ToNot(HaveOccurred())

						stopMessage, err := models.NewStopMessageFromJSON(b)
						Expect(err).ToNot(HaveOccurred())

						receivedStopMessages = append(receivedStopMessages, stopMessage)

						w.WriteHeader(500)
						fmt.Fprintln(w, "Throwing a 500")
					}))

					conf.CCInternalURL = server.URL
				})

				It("should return an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal("Sender failed. See logs for details."))
				})

				It("logs the specific error", func() {
					Expect(logger.LoggedSubjects()).To(ContainElement("Cloud Controller did not accept stop message"))

					expectedError := errors.New("Throwing a 500\n")
					Expect(logger.LoggedErrors()).To(ContainElement(expectedError))
				})

				It("should not increment the metrics", func() {
					Expect(pendingStartMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(HaveLen(0))
				})
			})
		})

		Context("When the message has already been sent", func() {
			BeforeEach(func() {
				sentOn = 130
				keepAliveTime = 30
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not increment the metrics", func() {
				Expect(pendingStartMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(HaveLen(0))
			})

			Context("and the keep alive has elapsed", func() {
				BeforeEach(func() {
					timeProvider = fakeclock.NewFakeClock(time.Unix(160, 0))
				})

				It("should delete the message and not send it", func() {
					messages, _ := store.GetPendingStopMessages()
					Expect(messages).To(BeEmpty())
					Expect(receivedStopMessages).To(HaveLen(0))
				})
			})

			Context("and the keep alive has not elapsed", func() {
				BeforeEach(func() {
					timeProvider = fakeclock.NewFakeClock(time.Unix(159, 0))
				})

				It("should neither delete the message nor send it", func() {
					messages, _ := store.GetPendingStopMessages()
					Expect(messages).To(HaveLen(1))

					Expect(receivedStopMessages).To(HaveLen(0))
				})
			})
		})
	})

	Describe("Verifying that start messages should be sent", func() {
		var err error
		var indexToStart int
		var pendingMessage models.PendingStartMessage
		var skipVerification bool

		JustBeforeEach(func() {
			timeProvider = fakeclock.NewFakeClock(time.Unix(130, 0))
			pendingMessage = models.NewPendingStartMessage(time.Unix(100, 0), 30, 10, app.AppGuid, app.AppVersion, indexToStart, 1.0, models.PendingStartMessageReasonInvalid)
			pendingMessage.SentOn = 0
			pendingMessage.SkipVerification = skipVerification
			store.SavePendingStartMessages(
				pendingMessage,
			)
			startMessages = append(startMessages, pendingMessage)

			err = sender.Send(timeProvider, apps, startMessages, stopMessages)
		})

		BeforeEach(func() {
			err = nil
			indexToStart = 0
			skipVerification = false
		})

		assertMessageWasNotSent := func() {
			It("should ignore the keep-alive and delete the start message from queue", func() {
				messages, _ := store.GetPendingStartMessages()
				Expect(messages).To(HaveLen(0))
			})

			It("should not send the start message", func() {
				Expect(receivedStartMessages).To(HaveLen(0))
			})

			It("should not increment the metrics", func() {
				Expect(pendingStartMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(HaveLen(0))
			})
		}

		assertMessageWasSent := func() {
			It("should honor the keep alive of the start message", func() {
				messages, _ := store.GetPendingStartMessages()
				Expect(messages).To(HaveLen(1))
				for _, message := range messages {
					Expect(message.SentOn).To(BeNumerically("==", 130))
				}
			})

			It("should send the start message", func() {
				Expect(receivedStartMessages).To(HaveLen(1))
				Expect(receivedStartMessages[0]).To(Equal(models.StartMessage{
					AppGuid:       app.AppGuid,
					AppVersion:    app.AppVersion,
					InstanceIndex: 0,
					MessageId:     pendingMessage.MessageId,
				}))
			})

			It("should increment the metrics", func() {
				Expect(pendingStartMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(ContainElement(pendingMessage))
			})
		}

		Context("When the app is still desired", func() {
			var desired models.DesiredAppState

			BeforeEach(func() {
				desired = app.DesiredState(1)
				apps[desired.StoreKey()] = models.NewApp(app.AppGuid, app.AppVersion, desired, nil, nil)
			})

			Context("when the index-to-start is within the # of desired instances", func() {
				BeforeEach(func() {
					indexToStart = 0
				})

				Context("when there are no running instances at all for that app", func() {
					assertMessageWasSent()
				})

				Context("when there is no running instance reporting at that index", func() {
					BeforeEach(func() {
						heartbeat := dea.HeartbeatWith(
							app.InstanceAtIndex(1).Heartbeat(),
							app.InstanceAtIndex(2).Heartbeat(),
						)
						apps[desired.StoreKey()] = models.NewApp(app.AppGuid, app.AppVersion, desired, heartbeat.InstanceHeartbeats, nil)
					})
					assertMessageWasSent()
				})

				Context("when there are crashed instances reporting at that index", func() {
					BeforeEach(func() {
						heartbeat := dea.HeartbeatWith(
							app.CrashedInstanceHeartbeatAtIndex(0),
							app.CrashedInstanceHeartbeatAtIndex(0),
							app.InstanceAtIndex(1).Heartbeat(),
							app.InstanceAtIndex(2).Heartbeat(),
						)
						apps[desired.StoreKey()] = models.NewApp(app.AppGuid, app.AppVersion, desired, heartbeat.InstanceHeartbeats, nil)
					})

					assertMessageWasSent()
				})

				Context("when there *is* a running instance reporting at that index", func() {
					BeforeEach(func() {
						heartbeat := dea.HeartbeatWith(
							app.InstanceAtIndex(0).Heartbeat(),
						)
						apps[desired.StoreKey()] = models.NewApp(app.AppGuid, app.AppVersion, desired, heartbeat.InstanceHeartbeats, nil)
					})

					assertMessageWasNotSent()
				})
			})

			Context("when the index-to-start is beyond the # of desired instances", func() {
				BeforeEach(func() {
					indexToStart = 1
				})

				assertMessageWasNotSent()
			})
		})

		Context("When the app is no longer desired", func() {
			assertMessageWasNotSent()
		})

		Context("when the message fails verification", func() {
			assertMessageWasNotSent()

			Context("but the message is marked with SkipVerification", func() {
				BeforeEach(func() {
					skipVerification = true
				})

				assertMessageWasSent()
			})
		})
	})

	Describe("Verifying that stop messages should be sent", func() {
		var err error
		var indexToStop int
		var pendingMessage models.PendingStopMessage
		var desired models.DesiredAppState

		JustBeforeEach(func() {
			timeProvider = fakeclock.NewFakeClock(time.Unix(130, 0))
			pendingMessage = models.NewPendingStopMessage(time.Unix(100, 0), 30, 10, app.AppGuid, app.AppVersion, app.InstanceAtIndex(indexToStop).InstanceGuid, models.PendingStopMessageReasonInvalid)
			pendingMessage.SentOn = 0
			store.SavePendingStopMessages(
				pendingMessage,
			)
			stopMessages = append(stopMessages, pendingMessage)

			err = sender.Send(timeProvider, apps, startMessages, stopMessages)
		})

		BeforeEach(func() {
			indexToStop = 0
		})

		assertMessageWasNotSent := func() {
			It("should ignore the keep-alive and delete the stop message from queue", func() {
				messages, _ := store.GetPendingStopMessages()
				Expect(messages).To(HaveLen(0))
			})

			It("should not send the stop message", func() {
				Expect(receivedStopMessages).To(HaveLen(0))
			})

			It("should not increment the metrics", func() {
				Expect(pendingStartMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(HaveLen(0))
			})
		}

		assertMessageWasSent := func(indexToStop int, isDuplicate bool) {
			It("should honor the keep alive of the stop message", func() {
				messages, _ := store.GetPendingStopMessages()
				Expect(messages).To(HaveLen(1))
				for _, message := range messages {
					Expect(message.SentOn).To(BeNumerically("==", 130))
				}
			})

			It("should send the stop message", func() {
				Expect(receivedStopMessages).To(HaveLen(1))
				message := receivedStopMessages[0]
				Expect(message).To(Equal(models.StopMessage{
					AppGuid:       app.AppGuid,
					AppVersion:    app.AppVersion,
					InstanceIndex: indexToStop,
					InstanceGuid:  app.InstanceAtIndex(indexToStop).InstanceGuid,
					IsDuplicate:   isDuplicate,
					MessageId:     pendingMessage.MessageId,
				}))
			})

			It("should increment the metrics", func() {
				Expect(pendingStopMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(ContainElement(pendingMessage))
			})
		}

		Context("When the app is still desired", func() {
			BeforeEach(func() {
				desired = app.DesiredState(1)
				apps[desired.StoreKey()] = models.NewApp(app.AppGuid, app.AppVersion, desired, nil, nil)
			})

			Context("When instance is still running", func() {
				BeforeEach(func() {
					heartbeat := dea.HeartbeatWith(
						app.InstanceAtIndex(0).Heartbeat(),
						app.InstanceAtIndex(1).Heartbeat(),
					)
					apps[desired.StoreKey()] = models.NewApp(app.AppGuid, app.AppVersion, desired, heartbeat.InstanceHeartbeats, nil)
				})

				Context("When index-to-stop is within the number of desired instances", func() {
					BeforeEach(func() {
						indexToStop = 0
					})

					Context("When there are other running instances on the index", func() {
						BeforeEach(func() {
							instance := app.InstanceAtIndex(0)
							instance.InstanceGuid = models.Guid()

							heartbeat := dea.HeartbeatWith(
								app.InstanceAtIndex(0).Heartbeat(),
								app.InstanceAtIndex(1).Heartbeat(),
								instance.Heartbeat(),
							)
							createdApp := models.NewApp(app.AppGuid, app.AppVersion, desired, heartbeat.InstanceHeartbeats, nil)
							apps[desired.StoreKey()] = createdApp
						})

						assertMessageWasSent(0, true)
					})

					Context("when there are other, crashed, instances on the index, and no running instances", func() {
						BeforeEach(func() {
							heartbeat := dea.HeartbeatWith(
								app.InstanceAtIndex(0).Heartbeat(),
								app.InstanceAtIndex(1).Heartbeat(),
								app.CrashedInstanceHeartbeatAtIndex(0),
							)
							apps[desired.StoreKey()] = models.NewApp(app.AppGuid, app.AppVersion, desired, heartbeat.InstanceHeartbeats, nil)
						})

						assertMessageWasNotSent()
					})

					Context("When there are no other running instances on the index", func() {
						assertMessageWasNotSent()
					})
				})

				Context("When index-to-stop is beyond the number of desired instances", func() {
					BeforeEach(func() {
						indexToStop = 1
					})

					assertMessageWasSent(1, false)
				})
			})

			Context("When the instance-to-stop is evacuating", func() {
				BeforeEach(func() {
					heartbeat := app.InstanceAtIndex(0).Heartbeat()
					heartbeat.State = models.InstanceStateEvacuating
					deaHeartbeat := dea.HeartbeatWith(
						heartbeat,
						app.InstanceAtIndex(1).Heartbeat(),
					)
					apps[desired.StoreKey()] = models.NewApp(app.AppGuid, app.AppVersion, desired, deaHeartbeat.InstanceHeartbeats, nil)
				})

				assertMessageWasSent(0, true)
			})

			Context("When instance is not running", func() {
				assertMessageWasNotSent()
			})
		})

		Context("When the app is no longer desired", func() {
			Context("when the instance is still running", func() {
				BeforeEach(func() {
					heartbeat := app.Heartbeat(2)
					desired = app.DesiredState(0)
					apps[desired.StoreKey()] = models.NewApp(app.AppGuid, app.AppVersion, desired, heartbeat.InstanceHeartbeats, nil)
				})
				assertMessageWasSent(0, false)
			})

			Context("when the instance is not running", func() {
				assertMessageWasNotSent()
			})
		})
	})

	Context("When there are multiple start and stop messages in the queue", func() {
		var invalidStartMessages, validStartMessages, expiredStartMessages []models.PendingStartMessage

		BeforeEach(func() {
			conf, _ = config.DefaultConfig()
			conf.CCInternalURL = server.URL
			conf.SenderMessageLimit = 20

			sender = New(store, metricsAccountant, conf, messageBus, fakelogger.NewFakeLogger(), timeProvider)

			for i := 0; i < 40; i += 1 {
				a := appfixture.NewAppFixture()
				desired := a.DesiredState(1)
				apps[desired.StoreKey()] = models.NewApp(a.AppGuid, a.AppVersion, desired, []models.InstanceHeartbeat{a.InstanceAtIndex(1).Heartbeat()}, nil)

				//only some of these should be sent:
				validStartMessage := models.NewPendingStartMessage(time.Unix(100, 0), 30, 0, a.AppGuid, a.AppVersion, 0, float64(i)/40.0, models.PendingStartMessageReasonInvalid)
				validStartMessages = append(validStartMessages, validStartMessage)
				store.SavePendingStartMessages(
					validStartMessage,
				)
				startMessages = append(startMessages, validStartMessage)

				//all of these should be deleted:
				invalidStartMessage := models.NewPendingStartMessage(time.Unix(100, 0), 30, 0, a.AppGuid, a.AppVersion, 1, 1.0, models.PendingStartMessageReasonInvalid)
				invalidStartMessages = append(invalidStartMessages, invalidStartMessage)
				store.SavePendingStartMessages(
					invalidStartMessage,
				)
				startMessages = append(startMessages, invalidStartMessage)

				//all of these should be deleted:
				expiredStartMessage := models.NewPendingStartMessage(time.Unix(100, 0), 0, 20, a.AppGuid, a.AppVersion, 2, 1.0, models.PendingStartMessageReasonInvalid)
				expiredStartMessage.SentOn = 100
				expiredStartMessages = append(expiredStartMessages, expiredStartMessage)
				store.SavePendingStartMessages(
					expiredStartMessage,
				)
				startMessages = append(startMessages, expiredStartMessage)

				stopMessage := models.NewPendingStopMessage(time.Unix(100, 0), 30, 0, a.AppGuid, a.AppVersion, a.InstanceAtIndex(1).InstanceGuid, models.PendingStopMessageReasonInvalid)
				store.SavePendingStopMessages(
					stopMessage,
				)
				stopMessages = append(stopMessages, stopMessage)
			}

			timeProvider = fakeclock.NewFakeClock(time.Unix(130, 0))
			err := sender.Send(timeProvider, apps, startMessages, stopMessages)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should limit the number of start messages that it sends", func() {
			remainingStartMessages, _ := store.GetPendingStartMessages()
			Expect(remainingStartMessages).To(HaveLen(20))
			Expect(receivedStartMessages).To(HaveLen(20))
			Expect(pendingStartMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(HaveLen(20))

			for _, remainingStartMessage := range remainingStartMessages {
				Expect(validStartMessages).To(ContainElement(remainingStartMessage))
				Expect(remainingStartMessage.Priority).To(BeNumerically("<=", 0.5))
			}
		})

		It("should delete all the invalid start messages", func() {
			remainingStartMessages, _ := store.GetPendingStartMessages()
			for _, invalidStartMessage := range invalidStartMessages {
				Expect(remainingStartMessages).NotTo(ContainElement(invalidStartMessage))
			}
		})

		It("should delete all the expired start messages", func() {
			remainingStartMessages, _ := store.GetPendingStartMessages()
			for _, expiredStartMessage := range expiredStartMessages {
				Expect(remainingStartMessages).NotTo(ContainElement(expiredStartMessage))
			}
		})

		It("should send all the stop messages, as they are cheap to handle", func() {
			remainingStopMessages, _ := store.GetPendingStopMessages()
			Expect(remainingStopMessages).To(BeEmpty())
			Expect(receivedStopMessages).To(HaveLen(40))
			Expect(pendingStopMessages(metricsAccountant.IncrementSentMessageMetricsArgsForCall(0))).To(HaveLen(40))
		})
	})
})

func pendingStartMessages(starts []models.PendingStartMessage, _ []models.PendingStopMessage) []models.PendingStartMessage {
	return starts
}

func pendingStopMessages(_ []models.PendingStartMessage, stops []models.PendingStopMessage) []models.PendingStopMessage {
	return stops
}
