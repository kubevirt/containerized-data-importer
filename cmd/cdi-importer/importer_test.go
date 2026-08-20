package main

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("waitForReadyFile", func() {
	It("waits long enough to outlast a container image pull", func() {
		Expect(readyFileTimeout).To(Equal(30 * time.Minute))
	})

	Context("awaitReadyFile", func() {
		var readyFile string

		BeforeEach(func() {
			readyFile = filepath.Join(GinkgoT().TempDir(), "ready")
		})

		It("succeeds when the file is already present", func() {
			Expect(os.WriteFile(readyFile, []byte("disk.img"), 0600)).To(Succeed())

			Expect(awaitReadyFile(readyFile, time.Second, time.Millisecond)).To(BeTrue())
		})

		It("succeeds when the file appears while waiting", func() {
			go func() {
				defer GinkgoRecover()
				time.Sleep(20 * time.Millisecond)
				Expect(os.WriteFile(readyFile, []byte("disk.img"), 0600)).To(Succeed())
			}()

			Expect(awaitReadyFile(readyFile, time.Minute, time.Millisecond)).To(BeTrue())
		})

		It("gives up once the timeout elapses", func() {
			Expect(awaitReadyFile(readyFile, 10*time.Millisecond, time.Millisecond)).To(BeFalse())
		})

		It("waits for the whole timeout before giving up", func() {
			const timeout = 50 * time.Millisecond
			start := time.Now()

			Expect(awaitReadyFile(readyFile, timeout, time.Millisecond)).To(BeFalse())

			Expect(time.Since(start)).To(BeNumerically(">=", timeout))
		})
	})
})
