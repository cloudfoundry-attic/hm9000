package cfcomponent

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/cloudfoundry/gosteno"
	"github.com/cloudfoundry/hm9000/cfcomponent/instrumentation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cfcomponent", func() {
	var logger *gosteno.Logger

	BeforeEach(func() {
		gosteno.EnterTestMode(gosteno.LOG_DEBUG)
		logger = gosteno.NewLogger("testlogger")
	})

	It("ip address default", func() {
		component, err := NewComponent(logger, "loggregator", 0, GoodHealthMonitor{}, 0, nil, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(component.IpAddress).NotTo(BeEmpty())
		Expect(component.IpAddress).NotTo(Equal("0.0.0.0"))
		Expect(component.IpAddress).NotTo(Equal("127.0.0.1"))
	})

	It("status port default", func() {
		component, err := NewComponent(logger, "loggregator", 0, GoodHealthMonitor{}, 0, nil, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(component.StatusPort).NotTo(Equal(uint32(0)))
	})

	It("status credentials nil", func() {
		component, err := NewComponent(logger, "loggregator", 0, GoodHealthMonitor{}, 0, nil, nil)
		Expect(err).NotTo(HaveOccurred())
		credentials := component.StatusCredentials
		Expect(credentials).To(HaveLen(2))
		Expect(credentials[0]).NotTo(BeEmpty())
		Expect(credentials[1]).NotTo(BeEmpty())
	})

	It("status credentials default", func() {
		component, err := NewComponent(logger, "loggregator", 0, GoodHealthMonitor{}, 0, []string{"", ""}, nil)
		Expect(err).NotTo(HaveOccurred())
		credentials := component.StatusCredentials
		Expect(credentials).To(HaveLen(2))
		Expect(credentials[0]).NotTo(BeEmpty())
		Expect(credentials[1]).NotTo(BeEmpty())
	})

	It("GUID", func() {
		component, err := NewComponent(logger, "loggregator", 0, GoodHealthMonitor{}, 0, []string{"", ""}, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(component.UUID).NotTo(BeEmpty())
	})

	It("good healthz endpoint", func() {
		component := &Component{
			Logger:            logger,
			HealthMonitor:     GoodHealthMonitor{},
			StatusPort:        7877,
			Type:              "loggregator",
			StatusCredentials: []string{"user", "pass"},
		}

		go component.StartMonitoringEndpoints()

		Eventually(func() bool {
			for _, r := range gosteno.GetMeTheGlobalTestSink().Records() {
				if strings.HasPrefix(r.Message, "Starting endpoints for component") {
					return true
				}
			}
			return false
		}).Should(BeTrue())

		req, err := http.NewRequest("GET", "http://localhost:7877/healthz", nil)
		Expect(err).NotTo(HaveOccurred())

		var resp *http.Response
		Eventually(func() error { resp, err = http.DefaultClient.Do(req); return err }).Should(Succeed())

		Expect(200).To(Equal(resp.StatusCode))
		Expect("text/plain").To(Equal(resp.Header.Get("Content-Type")))
		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect("ok").To(Equal(string(body)))
	})

	It("bad healthz endpoint", func() {
		component := &Component{
			Logger:            logger,
			HealthMonitor:     BadHealthMonitor{},
			StatusPort:        9878,
			Type:              "loggregator",
			StatusCredentials: []string{"user", "pass"},
		}

		go component.StartMonitoringEndpoints()

		req, err := http.NewRequest("GET", "http://localhost:9878/healthz", nil)
		Expect(err).NotTo(HaveOccurred())

		var resp *http.Response
		Eventually(func() error { resp, err = http.DefaultClient.Do(req); return err }).Should(Succeed())

		Expect(200).To(Equal(resp.StatusCode))
		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect("bad").To(Equal(string(body)))
	})

	It("panic when failing to monitor endpoints", func() {
		component := &Component{
			Logger:            logger,
			HealthMonitor:     GoodHealthMonitor{},
			StatusPort:        7879,
			Type:              "loggregator",
			StatusCredentials: []string{"user", "pass"},
		}

		finishChan := make(chan bool)

		go func() {
			err := component.StartMonitoringEndpoints()
			Expect(err).NotTo(HaveOccurred())
		}()
		time.Sleep(50 * time.Millisecond)

		go func() {
			err := component.StartMonitoringEndpoints()
			Expect(err).To(HaveOccurred())
			finishChan <- true
		}()

		<-finishChan
	})

	It("varz requires basic auth", func() {
		tags := map[string]interface{}{"tagName1": "tagValue1", "tagName2": "tagValue2"}
		component := &Component{
			Logger:            logger,
			HealthMonitor:     GoodHealthMonitor{},
			StatusPort:        1234,
			IpAddress:         "127.0.0.1",
			Type:              "loggregator",
			StatusCredentials: []string{"user", "pass"},
			Instrumentables: []instrumentation.Instrumentable{
				testInstrumentable{
					"agentListener",
					[]instrumentation.Metric{
						instrumentation.Metric{Name: "messagesReceived", Value: 2004},
						instrumentation.Metric{Name: "queueLength", Value: 5, Tags: tags},
					},
				},
				testInstrumentable{
					"cfSinkServer",
					[]instrumentation.Metric{
						instrumentation.Metric{Name: "activeSinkCount", Value: 3},
					},
				},
			},
		}

		go component.StartMonitoringEndpoints()

		req, err := http.NewRequest("GET", "http://localhost:1234/varz", nil)
		Expect(err).NotTo(HaveOccurred())

		var resp *http.Response
		Eventually(func() error { resp, err = http.DefaultClient.Do(req); return err }).Should(Succeed())

		Expect(401).To(Equal(resp.StatusCode))
	})

	It("varz endpoint", func() {
		tags := map[string]interface{}{"tagName1": "tagValue1", "tagName2": "tagValue2"}
		component := &Component{
			Logger:            logger,
			HealthMonitor:     GoodHealthMonitor{},
			StatusPort:        1234,
			IpAddress:         "127.0.0.1",
			Type:              "loggregator",
			StatusCredentials: []string{"user", "pass"},
			Instrumentables: []instrumentation.Instrumentable{
				testInstrumentable{
					"agentListener",
					[]instrumentation.Metric{
						instrumentation.Metric{Name: "messagesReceived", Value: 2004},
						instrumentation.Metric{Name: "queueLength", Value: 5, Tags: tags},
					},
				},
				testInstrumentable{
					"cfSinkServer",
					[]instrumentation.Metric{
						instrumentation.Metric{Name: "activeSinkCount", Value: 3},
					},
				},
			},
		}

		go component.StartMonitoringEndpoints()

		req, err := http.NewRequest("GET", "http://localhost:1234/varz", nil)
		Expect(err).NotTo(HaveOccurred())

		req.SetBasicAuth(component.StatusCredentials[0], component.StatusCredentials[1])

		var resp *http.Response
		Eventually(func() error { resp, err = http.DefaultClient.Do(req); return err }).Should(Succeed())

		memStats := new(runtime.MemStats)
		runtime.ReadMemStats(memStats)

		Expect(200).To(Equal(resp.StatusCode))
		Expect("application/json").To(Equal(resp.Header.Get("Content-Type")))
		body, err := ioutil.ReadAll(resp.Body)
		Expect(err).NotTo(HaveOccurred())

		expected := map[string]interface{}{
			"name":          "loggregator",
			"numCPUS":       runtime.NumCPU(),
			"numGoRoutines": runtime.NumGoroutine(),
			"memoryStats": map[string]interface{}{
				"numBytesAllocatedHeap":  int(memStats.HeapAlloc),
				"numBytesAllocatedStack": int(memStats.StackInuse),
				"numBytesAllocated":      int(memStats.Alloc),
				"numMallocs":             int(memStats.Mallocs),
				"numFrees":               int(memStats.Frees),
				"lastGCPauseTimeNS":      int(memStats.PauseNs[(memStats.NumGC+255)%256]),
			},
			"tags": map[string]string{
				"ip": "something",
			},
			"contexts": []interface{}{
				map[string]interface{}{
					"name": "agentListener",
					"metrics": []interface{}{
						map[string]interface{}{
							"name":  "messagesReceived",
							"value": float64(2004),
						},
						map[string]interface{}{
							"name":  "queueLength",
							"value": float64(5),
							"tags": map[string]interface{}{
								"tagName1": "tagValue1",
								"tagName2": "tagValue2",
							},
						},
					},
				},
				map[string]interface{}{
					"name": "cfSinkServer",
					"metrics": []interface{}{
						map[string]interface{}{
							"name":  "activeSinkCount",
							"value": float64(3),
						},
					},
				},
			},
		}

		var actualMap map[string]interface{}
		json.Unmarshal(body, &actualMap)
		Expect(actualMap).To(HaveKey("tags"))
		Expect(actualMap["contexts"]).To(Equal(expected["contexts"]))
		Expect(actualMap["name"]).To(Equal(expected["name"]))
		Expect(actualMap["numCPUS"]).To(BeNumerically("==", expected["numCPUS"]))
		Expect(actualMap["numGoRoutines"]).To(BeNumerically("==", expected["numGoRoutines"]))
		Expect(actualMap).To(HaveKey("memoryStats"))
		Expect(actualMap["memoryStats"]).NotTo(BeEmpty())
	})
})

type GoodHealthMonitor struct{}

func (hm GoodHealthMonitor) Ok() bool {
	return true
}

type BadHealthMonitor struct{}

func (hm BadHealthMonitor) Ok() bool {
	return false
}

type testInstrumentable struct {
	name    string
	metrics []instrumentation.Metric
}

func (t testInstrumentable) Emit() instrumentation.Context {
	return instrumentation.Context{Name: t.name, Metrics: t.metrics}
}
