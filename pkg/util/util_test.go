package util

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("Util", func() {
	It("Should match RandAlphaNum", func() {
		got := RandAlphaNum(8)
		Expect(got).To(HaveLen(8))
		Expect(regexp.MustCompile(pattern).Match([]byte(got))).To(BeTrue())
	})

	DescribeTable("Round down", func(input, multiple, expectedResult int64) {
		result := RoundDown(input, multiple)
		Expect(result).To(Equal(expectedResult))
	},
		Entry("Round down 513 to nearest multiple of 512", int64(513), int64(512), int64(512)),
		Entry("Round down 512 to nearest multiple of 512", int64(512), int64(512), int64(512)),
		Entry("Round down 510 to nearest multiple of 512", int64(510), int64(512), int64(0)),
		Entry("Round down 0 to nearest multiple of 512", int64(0), int64(512), int64(0)),
		Entry("Round down 513 to nearest multiple of 2", int64(513), int64(2), int64(512)),
		Entry("Round down 512 to nearest multiple of 2", int64(512), int64(2), int64(512)),
		Entry("Round down 510 to nearest multiple of 2", int64(510), int64(2), int64(510)),
	)

	DescribeTable("Round up", func(input, multiple, expectedResult int64) {
		result := RoundUp(input, multiple)
		Expect(result).To(Equal(expectedResult))
	},
		Entry("Round up 513 to nearest multiple of 512", int64(513), int64(512), int64(1024)),
		Entry("Round up 512 to nearest multiple of 512", int64(512), int64(512), int64(512)),
		Entry("Round up 510 to nearest multiple of 512", int64(510), int64(512), int64(512)),
		Entry("Round up 0 to nearest multiple of 512", int64(0), int64(512), int64(0)),
		Entry("Round up 513 to nearest multiple of 2", int64(513), int64(2), int64(514)),
		Entry("Round up 512 to nearest multiple of 2", int64(512), int64(2), int64(512)),
		Entry("Round up 510 to nearest multiple of 2", int64(510), int64(2), int64(510)),
	)

	DescribeTable("Find Namespace", func(inputFile, expectedResult string) {
		result := getNamespace(inputFile)
		Expect(result).To(Equal(expectedResult))
	},
		Entry("Valid namespace", filepath.Join(fileDir, "namespace.txt"), "test-namespace"),
		Entry("Invalid file", "doesnotexist", "cdi"),
	)
})

var _ = Describe("GetNameSpace", func() {
	It("Report default namespace outside container", func() {
		Expect("cdi").To(Equal(GetNamespace()))
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

		_, err = ParseEnvVar("value3", true)
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

var _ = Describe("Usable Space calculation", func() {

	const (
		Mi              = int64(1024 * 1024)
		Gi              = 1024 * Mi
		noOverhead      = float64(0)
		defaultOverhead = float64(0.055)
		largeOverhead   = float64(0.75)
	)
	DescribeTable("getusablespace should return properly aligned sizes,", func(virtualSize int64, overhead float64) {
		for i := virtualSize - 1024; i < virtualSize+1024; i++ {
			// Requested space is virtualSize rounded up to 1Mi alignment / (1 - overhead) rounded up
			requestedSpace := int64(float64(RoundUp(i, DefaultAlignBlockSize)+1) / (1 - overhead))
			if i <= virtualSize {
				Expect(GetUsableSpace(overhead, requestedSpace)).To(Equal(virtualSize))
			} else {
				Expect(GetUsableSpace(overhead, requestedSpace)).To(Equal(virtualSize + Mi))
			}
		}
	},
		Entry("1Mi virtual size, 0 overhead to be 1Mi if <= 1Mi and 2Mi if > 1Mi", Mi, noOverhead),
		Entry("1Mi virtual size, default overhead to be 1Mi if <= 1Mi and 2Mi if > 1Mi", Mi, defaultOverhead),
		Entry("1Mi virtual size, large overhead to be 1Mi if <= 1Mi and 2Mi if > 1Mi", Mi, largeOverhead),
		Entry("40Mi virtual size, 0 overhead to be 40Mi if <= 1Mi and 41Mi if > 40Mi", 40*Mi, noOverhead),
		Entry("40Mi virtual size, default overhead to be 40Mi if <= 1Mi and 41Mi if > 40Mi", 40*Mi, defaultOverhead),
		Entry("40Mi virtual size, large overhead to be 40Mi if <= 40Mi and 41Mi if > 40Mi", 40*Mi, largeOverhead),
		Entry("1Gi virtual size, 0 overhead to be 1Gi if <= 1Gi and 2Gi if > 1Gi", Gi, noOverhead),
		Entry("1Gi virtual size, default overhead to be 1Gi if <= 1Gi and 2Gi if > 1Gi", Gi, defaultOverhead),
		Entry("1Gi virtual size, large overhead to be 1Gi if <= 1Gi and 2Gi if > 1Gi", Gi, largeOverhead),
		Entry("40Gi virtual size, 0 overhead to be 40Gi if <= 1Gi and 41Gi if > 40Gi", 40*Gi, noOverhead),
		Entry("40Gi virtual size, default overhead to be 40Gi if <= 1Gi and 41Gi if > 40Gi", 40*Gi, defaultOverhead),
		Entry("40Gi virtual size, large overhead to be 40Gi if <= 40Gi and 41Gi if > 40Gi", 40*Gi, largeOverhead),
	)
})
