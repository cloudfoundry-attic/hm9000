package desiredstatepoller

import (
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry/hm9000/models"
	"github.com/cloudfoundry/hm9000/test_helpers/app"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Desired State Server Response JSON", func() {
	var (
		a        app.App
		response desiredStateServerResponse
	)
	BeforeEach(func() {
		a = app.NewApp()

		desired, _ := json.Marshal(a.DesiredState(0))
		responseJson := fmt.Sprintf(`
        {
            "results":{"%s":%s},
            "bulk_token":{"id":17}
        }
        `, a.AppGuid, string(desired))

		var err error
		response, err = NewDesiredStateServerResponse([]byte(responseJson))
		Ω(err).ShouldNot(HaveOccured())
	})

	It("can parse from JSON", func() {
		Ω(response.Results).Should(HaveLen(1))
		Ω(response.Results[a.AppGuid]).Should(models.EqualDesiredState(a.DesiredState(0)))
		Ω(response.BulkToken.Id).Should(Equal(17))
	})

	It("can return the bulk_token representation", func() {
		Ω(response.BulkTokenRepresentation()).Should(Equal(`{"id":17}`))
	})

	Context("when the JSON can't be parsed", func() {
		It("should return an error", func() {
			_, err := NewDesiredStateServerResponse([]byte("{"))
			Ω(err).Should(HaveOccured())
		})
	})

	Describe("ToJson", func() {
		It("should return json that survives the round trip", func() {
			resurrectedResponse, err := NewDesiredStateServerResponse(response.ToJson())
			Ω(err).ShouldNot(HaveOccured())
			Ω(resurrectedResponse).Should(Equal(response))
		})
	})
})
