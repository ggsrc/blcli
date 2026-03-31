package blcli_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBlcli(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Blcli Suite")
}
