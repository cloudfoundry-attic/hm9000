package models_test

import (
	. "github.com/cloudfoundry/hm9000/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BasicAuthInfo", func() {
	Describe("basic auth encoding", func() {
		It("should encode the user and password in basic auth form", func() {
			info := BasicAuthInfo{"mcat", "testing"}
			Ω(info.Encode()).Should(Equal("Basic bWNhdDp0ZXN0aW5n"))
		})
	})
})
