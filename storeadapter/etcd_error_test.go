package storeadapter_test

import (
	. "github.com/cloudfoundry/hm9000/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ETCD Errors", func() {
	It("can make a KeyNotFoundError", func() {
		err := NewETCDError(ETCDErrorKeyNotFound)
		Ω(IsKeyNotFoundError(err)).Should(BeTrue())
	})
})
