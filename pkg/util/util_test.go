package util

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"

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

var _ = Describe("GetNameSpace", func() {
	It("Report default namespace outside container", func() {
		Expect(v1.NamespaceSystem).To(Equal(GetNamespace()))
	})
})

var _ = Describe("ParseEnv", func() {
	BeforeEach(func() {
		os.Setenv("value1", "value1")
		os.Setenv("value2", base64.StdEncoding.EncodeToString([]byte("value2")))
		os.Setenv("value3", "invalid --- *** &&&")
	})

	AfterEach(func() {
		os.Unsetenv("value1")
		os.Unsetenv("value2")
		os.Unsetenv("value3")
	})

	It("Parse environment unencoded variables", func() {
		result, err := ParseEnvVar("value1", false)
		Expect(result).To(Equal("value1"))
		Expect(err).ToNot(HaveOccurred())
		result, err = ParseEnvVar("value1", true)
		Expect(result).ToNot(Equal("value1"))
		Expect(err).To(HaveOccurred())

		result, err = ParseEnvVar("value2", false)
		Expect(result).ToNot(Equal("value2"))
		Expect(err).ToNot(HaveOccurred())
		result, err = ParseEnvVar("value2", true)
		Expect(result).To(Equal("value2"))
		Expect(err).ToNot(HaveOccurred())

		result, err = ParseEnvVar("value3", true)
		Expect(err).To(HaveOccurred())
	})
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
