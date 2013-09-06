package store

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ETCD Errors", func() {
	It("can make a KeyNotFoundError", func() {
		err := ETCDError{reason: ETCDErrorKeyNotFound}
		Ω(IsKeyNotFoundError(err)).Should(BeTrue())
	})
})
