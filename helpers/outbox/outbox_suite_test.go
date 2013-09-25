package outbox_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestOutbox(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Outbox Suite")
}
