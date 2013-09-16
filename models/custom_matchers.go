package models

import (
	"github.com/onsi/gomega"

	"fmt"
)

func EqualDesiredState(expected DesiredAppState) gomega.OmegaMatcher {
	return &desiredStateMatcher{expected: expected}
}

type desiredStateMatcher struct {
	expected DesiredAppState
}

func (m *desiredStateMatcher) Match(actual interface{}) (success bool, message string, err error) {
	desiredState, ok := actual.(DesiredAppState)
	if !ok {
		return false, "", fmt.Errorf("DesiredStateMatcher expects a DesiredAppState, got %T instead", actual)
	}

	if m.expected.Equal(desiredState) {
		return true, fmt.Sprintf("Expected\n\t%#v\nnot to equal\n\t%#v", desiredState, m.expected), nil
	} else {
		return false, fmt.Sprintf("Expected\n\t%#v\nto equal\n\t%#v", desiredState, m.expected), nil
	}
}
