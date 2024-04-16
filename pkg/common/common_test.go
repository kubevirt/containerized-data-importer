package common

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var _ = Describe("TerminationMessage", func() {
	It("Should successfully serialize a TerminationMessage", func() {
		termMsg := TerminationMessage{
			VddkInfo: &VddkInfo{
				Version: "testversion",
				Host:    "testhost",
			},
			Labels: map[string]string{
				"testlabel": "testvalue",
			},
			PreallocationApplied: ptr.To(true),
		}

		serialized, err := termMsg.String()
		Expect(err).ToNot(HaveOccurred())
		Expect(serialized).To(Equal(`{"preallocationApplied":true,"vddkInfo":{"Version":"testversion","Host":"testhost"},"labels":{"testlabel":"testvalue"}}`))
	})

	It("Should fail if serialized data is longer than 4096 bytes", func() {
		const length = 5000
		const serializationOffset = 19

		termMsg := TerminationMessage{
			Labels: map[string]string{},
		}
		for i := 0; i < length-serializationOffset; i++ {
			termMsg.Labels["t"] += "c"
		}

		_, err := termMsg.String()
		Expect(err).To(MatchError(fmt.Sprintf("Termination message length %d exceeds maximum length of 4096 bytes", length)))
	})
})
