package auth

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cfcomponent auth", func() {
	It("requires basic auth", func() {
		w := performMockRequest("realm")
		Expect(w.Code).To(Equal(401))
		Expect(w.HeaderMap.Get("WWW-Authenticate")).To(Equal(`Basic realm="realm"`))
		Expect(w.Body.String()).To(Equal("401 Unauthorized"))
	})

	It("requires prompts with given realm", func() {
		w := performMockRequest("myrealm")
		Expect(w.HeaderMap.Get("WWW-Authenticate")).To(Equal(`Basic realm="myrealm"`))
	})

	It("fails with bad credentials", func() {
		w := performMockRequest("realm", "baduser", "badpassword")
		Expect(w.Code).To(Equal(401))
	})

	It("succeeds with good credentials", func() {
		w := performMockRequest("realm", "user", "password")
		Expect(w.Code).To(Equal(200))
		Expect(w.Body.String()).To(Equal("OK"))
	})
})

func performMockRequest(args ...string) *httptest.ResponseRecorder {
	handler := func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("OK"))
	}
	auth := NewBasicAuth(args[0], []string{"user", "password"})
	wrappedHandler := auth.Wrap(handler)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "http://foo/path", nil)
	if len(args) == 3 {
		req.SetBasicAuth(args[1], args[2])
	}
	wrappedHandler(w, req)
	return w
}
