package storecassandra_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestStorecassandra(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storecassandra Suite")
}
