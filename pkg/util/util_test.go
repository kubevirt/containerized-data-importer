package util

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

const pattern = "^[a-zA-Z0-9]+$"
const TestImagesDir = "../../tests/images"

var fileDir, _ = filepath.Abs(TestImagesDir)

var _ = Describe("Util", func() {
	It("Should match RandAlphaNum", func() {
		got := RandAlphaNum(8)
		Expect(len(got)).To(Equal(8))
		Expect(regexp.MustCompile(pattern).Match([]byte(got))).To(BeTrue())
	})

	table.DescribeTable("Find Namespace", func(inputFile, expectedResult string) {
		result := getNamespace(inputFile)
		Expect(result).To(Equal(expectedResult))
	},
		table.Entry("Valid namespace", filepath.Join(fileDir, "namespace.txt"), "test-namespace"),
		table.Entry("Invalid file", "doesnotexist", v1.NamespaceSystem),
	)
})

var _ = Describe("Compare quantities", func() {
	It("Should properly compare quantities", func() {
		small := resource.NewScaledQuantity(int64(1000), 0)
		big := resource.NewScaledQuantity(int64(10000), 0)
		result := MinQuantity(small, big)
		Expect(result).To(Equal(*small))
		result = MinQuantity(big, small)
		Expect(result).To(Equal(*small))
	})
})

var _ = Describe("File system", func() {
	By("Checking current available space")
	var stat syscall.Statfs_t
	syscall.Statfs(".", &stat)
	blockSize := stat.Bsize
	fmt.Fprintf(GinkgoWriter, "INFO: Block size: %d\n", blockSize)

	table.DescribeTable("Write file near block size", func(fileSize, usedBlocks int64) {
		By("Checking current available space")
		var stat syscall.Statfs_t
		syscall.Statfs(".", &stat)
		currentAvailabeBlocks := int64(stat.Bavail)
		f, err := os.Create("smallerthanblock.txt")
		defer os.Remove("smallerthanblock.txt")
		Expect(err).NotTo(HaveOccurred())
		for i := int64(0); i < fileSize; i++ {
			f.WriteString("t")
		}
		fileInfo, err := f.Stat()
		fmt.Fprintf(GinkgoWriter, "INFO: Written file size: %d\n", fileInfo.Size())
		err = f.Close()
		Expect(err).NotTo(HaveOccurred())
		newAvailableSpace := GetAvailableSpace(".")
		Expect(newAvailableSpace).To(Equal((currentAvailabeBlocks - usedBlocks) * blockSize))
	},
		table.Entry("< block size", blockSize-int64(1), int64(1)),
		table.Entry("= block size", blockSize, int64(1)),
		table.Entry("> block size", blockSize+int64(1), int64(2)),
	)
})
