package mcat_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"code.cloudfoundry.org/cfhttp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Locking", func() {
	Describe("vieing for the lock", func() {
		Context("when two long-lived processes try to run", func() {
			It("one waits for the other to exit and then grabs the lock", func() {
				listenerA := cliRunner.StartSession("listen", 1)

				Eventually(listenerA, 10*time.Second).Should(gbytes.Say("acquire-lock-succeeded"))

				defer func() {
					listenerA.Interrupt().Wait(5 * time.Second)
				}()

				listenerB := cliRunner.StartSession("listen", 1)
				defer func() {
					listenerB.Interrupt().Wait(5 * time.Second)
				}()

				Eventually(listenerB, 10*time.Second).Should(gbytes.Say("acquiring-lock"))
				Consistently(listenerB).ShouldNot(gbytes.Say("acquire-lock-succeeded"))

				listenerA.Interrupt().Wait(5 * time.Second)

				Eventually(listenerA, 10*time.Second).Should(gbytes.Say("releasing-lock"))
				Eventually(listenerB, 10*time.Second).Should(gbytes.Say("acquire-lock-succeeded"))
			})
		})

		Context("when two polling processes try to run", func() {
			It("one waits for the other to exit and then grabs the lock", func() {
				analyzerA := cliRunner.StartSession("analyze", 1, "--poll")
				defer func() {
					analyzerA.Interrupt().Wait(11 * time.Second)
				}()

				Eventually(analyzerA, 10*time.Second).Should(gbytes.Say("acquire-lock-succeeded"))

				analyzerB := cliRunner.StartSession("analyze", 1, "--poll")
				defer func() {
					analyzerB.Interrupt().Wait(11 * time.Second)
				}()

				Eventually(analyzerB, 10*time.Second).Should(gbytes.Say("acquiring-lock"))
				Consistently(analyzerB).ShouldNot(gbytes.Say("acquire-lock-succeeded"))

				analyzerA.Interrupt().Wait(11 * time.Second)

				Eventually(analyzerB, 20*time.Second).Should(gbytes.Say("acquire-lock-succeeded"))
			})
		})
	})

	Context("when the lock disappears", func() {
		Context("long-lived processes", func() {
			It("should exit 197", func() {
				listenerA := cliRunner.StartSession("listen", 1)
				defer func() {
					listenerA.Interrupt().Wait(5 * time.Second)
				}()

				Eventually(listenerA, 10*time.Second).Should(gbytes.Say("acquire-lock-succeeded"))

				coordinator.ResetConsulRunner()

				Eventually(listenerA, 10*time.Second).Should(gbytes.Say("lost-lock"))
				Eventually(listenerA, 20*time.Second).Should(gexec.Exit(197))
			})
		})

		Context("polling processes", func() {
			It("should exit 197", func() {
				analyzerA := cliRunner.StartSession("analyze", 1, "--poll")
				defer func() {
					analyzerA.Interrupt().Wait(5 * time.Second)
				}()

				Eventually(analyzerA, 10*time.Second).Should(gbytes.Say("acquire-lock-succeeded"))

				// coordinator.StoreAdapter.Delete("/hm/locks")
				coordinator.ResetConsulRunner()

				Eventually(analyzerA, 10*time.Second).Should(gbytes.Say("lost-lock"))
				Eventually(analyzerA, 20*time.Second).Should(gexec.Exit(197))
			})
		})
	})

	Describe("route registration", func() {
		It("registers the service with consul", func() {
			listenerA := cliRunner.StartSession("listen", 1)

			Eventually(listenerA, 10*time.Second).Should(gbytes.Say("acquire-lock-succeeded"))

			defer func() {
				listenerA.Interrupt().Wait(5 * time.Second)
			}()

			client := cfhttp.NewStreamingClient()
			Eventually(func() string {
				rsp, err := client.Get(coordinator.ConsulRunner.URL() + "/v1/catalog/service/listener-hm9000")
				Expect(err).NotTo(HaveOccurred())
				defer rsp.Body.Close()
				Expect(rsp.StatusCode).To(Equal(http.StatusOK))
				bs, err := ioutil.ReadAll(rsp.Body)
				Expect(err).NotTo(HaveOccurred())
				services := []map[string]interface{}{}
				err = json.Unmarshal(bs, &services)
				Expect(err).NotTo(HaveOccurred())
				if len(services) != 1 {
					return ""
				}
				return services[0]["ServiceID"].(string)
			}).Should(MatchRegexp(`[[:alnum:]]{8}-[[:alnum:]]{4}-[[:alnum:]]{4}-[[:alnum:]]{4}-[[:alnum:]]{12}`))

			listenerA.Interrupt().Wait(5 * time.Second)
			Eventually(listenerA, 10*time.Second).Should(gexec.Exit())
		})
	})
})
