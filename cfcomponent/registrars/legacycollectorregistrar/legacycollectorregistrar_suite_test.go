package legacycollectorregistrar_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"testing"
)

func TestInstrumentation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Legacy Collector Registrar Suite")
}
