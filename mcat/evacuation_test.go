package mcat_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/testhelpers/appfixture"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/rata"
)

var _ = Describe("Evacuation and Shutdown", func() {
	var (
		dea appfixture.DeaFixture
		app appfixture.AppFixture
	)

	BeforeEach(func() {
		dea = appfixture.NewDeaFixture()
		app = dea.GetApp(0)
		simulator.SetCurrentHeartbeats(dea.HeartbeatWith(app.InstanceAtIndex(0).Heartbeat()))
		simulator.SetDesiredState(app.DesiredState(1))
		simulator.Tick(simulator.TicksToAttainFreshness, false)
	})

	Describe("Shutdown handling by the evacuator component", func() {
		Context("when a SHUTDOWN droplet.exited message comes in", func() {
			BeforeEach(func() {
				cliRunner.StartEvacuator(simulator.currentTimestamp)
				coordinator.MessageBus.Publish("droplet.exited", app.InstanceAtIndex(0).DropletExited(models.DropletExitedReasonDEAShutdown).ToJSON())
			})

			AfterEach(func() {
				cliRunner.StopEvacuator()
			})

			It("should immediately start the app", func() {
				simulator.Tick(1, false)
				Expect(startStopListener.StartCount()).To(Equal(1))
				startMsg := startStopListener.Start(0)
				Expect(startMsg.AppGuid).To(Equal(app.AppGuid))
				Expect(startMsg.AppVersion).To(Equal(app.AppVersion))
				Expect(startMsg.InstanceIndex).To(Equal(0))
			})
		})
	})

	Describe("Shutdown handling by the actual state listener", func() {
		Context("when an evacuation heartbeat is received", func() {
			var (
				hbClient      *http.Client
				sendHeartbeat *http.Request
			)

			BeforeEach(func() {
				cliRunner.StartListener(simulator.currentTimestamp)
				hbClient = &http.Client{}

				serverAddr := fmt.Sprintf("https://%s:%d", cliRunner.config.HttpHeartbeatServerAddress, cliRunner.config.HttpHeartbeatPort)
				routes := rata.Routes{{Method: "POST", Name: "dea_heartbeat_handler", Path: "/dea/heartbeat"}}
				requestGenerator := rata.NewRequestGenerator(serverAddr, routes)

				certFile, err := filepath.Abs("../testhelpers/fake_certs/hm9000_client.crt")
				Expect(err).NotTo(HaveOccurred())

				keyFile, err := filepath.Abs("../testhelpers/fake_certs/hm9000_client.key")
				Expect(err).NotTo(HaveOccurred())

				cert, err := tls.LoadX509KeyPair(certFile, keyFile)
				Expect(err).NotTo(HaveOccurred())

				caCertFile, err := filepath.Abs("../testhelpers/fake_certs/hm9000_ca.crt")
				Expect(err).NotTo(HaveOccurred())

				caCert, err := ioutil.ReadFile(caCertFile)
				Expect(err).NotTo(HaveOccurred())

				caCertPool := x509.NewCertPool()
				caCertPool.AppendCertsFromPEM(caCert)

				tlsConfig := &tls.Config{
					Certificates: []tls.Certificate{cert},
					RootCAs:      caCertPool,
				}

				tlsConfig.BuildNameToCertificate()

				tlsTransport := &http.Transport{
					TLSClientConfig: tlsConfig,
				}

				hbClient = &http.Client{
					Transport: tlsTransport,
				}

				dea = appfixture.NewDeaFixture()

				evacuatingHeartbeat := dea.Heartbeat(1)
				evacuatingHeartbeat.InstanceHeartbeats[0].State = models.InstanceStateEvacuating
				sendHeartbeat, err = requestGenerator.CreateRequest(
					"dea_heartbeat_handler",
					nil,
					bytes.NewBuffer(evacuatingHeartbeat.ToJSON()),
				)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				cliRunner.StopListener()
			})

			It("sends a start message for the evacuating app", func() {
				response, err := hbClient.Do(sendHeartbeat)
				Expect(err).NotTo(HaveOccurred())
				defer response.Body.Close()

				Expect(response.StatusCode).To(Equal(http.StatusAccepted))

				Eventually(startStopListener.StartCount).Should(Equal(1))
				startMsg := startStopListener.Start(0)
				Expect(startMsg).ToNot(BeNil())
				Expect(startMsg.AppGuid).To(Equal(dea.GetApp(0).AppGuid))
			})
		})
	})

	Describe("Deterministic evacuation", func() {
		Context("with http", func() {
			Context("when an app enters the evacuation state", func() {
				var evacuatingHeartbeat models.InstanceHeartbeat

				BeforeEach(func() {
					Expect(startStopListener.StartCount()).To(BeZero())
					Expect(startStopListener.StopCount()).To(BeZero())
					evacuatingHeartbeat = app.InstanceAtIndex(0).Heartbeat()
					evacuatingHeartbeat.State = models.InstanceStateEvacuating

					simulator.SetCurrentHeartbeats(dea.HeartbeatWith(evacuatingHeartbeat))
					simulator.Tick(1, true)
				})

				AfterEach(func() {
					cliRunner.StopListener()
				})

				It("should immediately start the app", func() {
					Eventually(startStopListener.StartCount, 5).Should(Equal(1))
					startMsg := startStopListener.Start(0)
					Expect(startMsg.AppGuid).To(Equal(app.AppGuid))
					Expect(startMsg.AppVersion).To(Equal(app.AppVersion))
					Expect(startMsg.InstanceIndex).To(Equal(0))
					Expect(startStopListener.StopCount()).To(BeZero())
				})

				Context("when the app starts", func() {
					BeforeEach(func() {
						startStopListener.Reset()
						runningHeartbeat := app.InstanceAtIndex(0).Heartbeat()
						runningHeartbeat.InstanceGuid = models.Guid()
						simulator.SetCurrentHeartbeats(&models.Heartbeat{
							DeaGuid:            "new-dea",
							InstanceHeartbeats: []models.InstanceHeartbeat{runningHeartbeat},
						})
						simulator.Tick(1, true)
					})

					It("should stop the evacuated instance", func() {
						Expect(startStopListener.StartCount()).To(BeZero())
						Eventually(startStopListener.StopCount, 5).Should(Equal(1))
						stopMsg := startStopListener.Stop(0)
						Expect(stopMsg.AppGuid).To(Equal(app.AppGuid))
						Expect(stopMsg.AppVersion).To(Equal(app.AppVersion))
						Expect(stopMsg.InstanceGuid).To(Equal(evacuatingHeartbeat.InstanceGuid))
					})
				})

				Context("when the hm9000 listener restarts", func() {
					It("does not send an additional start for the evacuating app instance", func() {
						simulator.Tick(1, true)

						Expect(startStopListener.StartCount()).To(Equal(1))
					})
				})
			})

			Context("with nats", func() {
				Context("when an app enters the evacuation state", func() {
					var evacuatingHeartbeat models.InstanceHeartbeat

					BeforeEach(func() {
						Expect(startStopListener.StartCount()).To(BeZero())
						Expect(startStopListener.StopCount()).To(BeZero())
						evacuatingHeartbeat = app.InstanceAtIndex(0).Heartbeat()
						evacuatingHeartbeat.State = models.InstanceStateEvacuating

						simulator.SetCurrentHeartbeats(dea.HeartbeatWith(evacuatingHeartbeat))
						simulator.Tick(1, false)
					})

					It("should immediately start the app", func() {
						Expect(startStopListener.StartCount()).To(Equal(1))
						startMsg := startStopListener.Start(0)
						Expect(startMsg.AppGuid).To(Equal(app.AppGuid))
						Expect(startMsg.AppVersion).To(Equal(app.AppVersion))
						Expect(startMsg.InstanceIndex).To(Equal(0))
						Expect(startStopListener.StopCount()).To(BeZero())
					})

					Context("when the app starts", func() {
						BeforeEach(func() {
							startStopListener.Reset()
							runningHeartbeat := app.InstanceAtIndex(0).Heartbeat()
							runningHeartbeat.InstanceGuid = models.Guid()
							simulator.SetCurrentHeartbeats(&models.Heartbeat{
								DeaGuid:            "new-dea",
								InstanceHeartbeats: []models.InstanceHeartbeat{runningHeartbeat},
							})
							simulator.Tick(1, false)
						})

						It("should stop the evacuated instance", func() {
							Expect(startStopListener.StartCount()).To(BeZero())
							Expect(startStopListener.StopCount()).To(Equal(1))
							stopMsg := startStopListener.Stop(0)
							Expect(stopMsg.AppGuid).To(Equal(app.AppGuid))
							Expect(stopMsg.AppVersion).To(Equal(app.AppVersion))
							Expect(stopMsg.InstanceGuid).To(Equal(evacuatingHeartbeat.InstanceGuid))
						})
					})
				})
			})
		})
	})
})
