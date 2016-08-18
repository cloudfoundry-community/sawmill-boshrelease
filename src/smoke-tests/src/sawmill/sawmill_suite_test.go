package sawmill

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSawmill(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sawmill Smoke Tests")
}
