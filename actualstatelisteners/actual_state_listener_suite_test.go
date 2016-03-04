package actualstatelisteners_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestActualStateListeners(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Actual State Handler Suite")
}
