package storeadapter_test

import (
	"errors"
	. "github.com/cloudfoundry/hm9000/storeadapter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Store Errors", func() {
	It("can make a KeyNotFound error", func() {
		err := NewStoreError(StoreErrorKeyNotFound)
		Ω(IsKeyNotFoundError(err)).Should(BeTrue())
		Ω(IsKeyNotFoundError(errors.New("foo"))).Should(BeFalse())
	})

	It("can make an IsDirectory error", func() {
		err := NewStoreError(StoreErrorIsDirectory)
		Ω(IsDirectoryError(err)).Should(BeTrue())
		Ω(IsDirectoryError(errors.New("foo"))).Should(BeFalse())
	})

	It("can make an IsNotDirectory error", func() {
		err := NewStoreError(StoreErrorIsNotDirectory)
		Ω(IsNotDirectoryError(err)).Should(BeTrue())
		Ω(IsNotDirectoryError(errors.New("foo"))).Should(BeFalse())
	})

	It("can make a Timeout error", func() {
		err := NewStoreError(StoreErrorTimeout)
		Ω(IsTimeoutError(err)).Should(BeTrue())
		Ω(IsTimeoutError(errors.New("foo"))).Should(BeFalse())
	})
})
