package models

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BasicAuthInfo", func() {
	Describe("creating from JSON", func() {
		It("can be created from JSON", func() {
			info, err := NewBasicAuthInfoFromJSON([]byte(`{"user":"mcat","password":"testing"}`))
			Ω(err).ShouldNot(HaveOccured())
			Ω(info.User).Should(Equal("mcat"))
			Ω(info.Password).Should(Equal("testing"))
		})

		Context("when the JSON fails to parse", func() {
			It("returns an error", func() {
				_, err := NewBasicAuthInfoFromJSON([]byte(`{`))
				Ω(err).Should(HaveOccured())
			})
		})
	})

	Describe("basic auth encoding", func() {
		It("should encode the user and password in basic auth form", func() {
			info, _ := NewBasicAuthInfoFromJSON([]byte(`{"user":"mcat","password":"testing"}`))
			Ω(info.Encode()).Should(Equal("Basic bWNhdDp0ZXN0aW5n"))
		})
	})
})
