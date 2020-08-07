/*
 * This file is part of the CDI project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2020 Red Hat, Inc.
 *
 */

package naming

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

var _ = Describe("GetName", func() {
	word63 := util.RandAlphaNum(63)
	word260 := util.RandAlphaNum(260)
	suffix250 := util.RandAlphaNum(250)

	It("Should namer differentiates names if suffix is dropped", func() {
		result := GetResourceName("base", util.RandAlphaNum(260))

		anotherDifferentResult := GetResourceName("base", util.RandAlphaNum(260))
		Expect(result).ToNot(Equal(anotherDifferentResult))
	})

	table.DescribeTable("getName", func(inputName, suffix string, resultMatcher types.GomegaMatcher) {
		result := GetResourceName(inputName, suffix)
		Expect(len(result)).To(BeNumerically("<=", 253))
		Expect(result).To(resultMatcher)
	},
		table.Entry("Should not changed short name that fits under limits ", "abc", "suffix", Equal("abc-suffix")),
		table.Entry("Should shorten name and join with -hash-suffix", word260, "suffix", HaveSuffix("-suffix")),
		table.Entry("Should shorten too long name dropping too long suffix", "abc123", suffix250, And(HavePrefix("abc123"), HaveLen(15))),
	)

	It("Should handle dot '.' correctly", func() {
		Expect(GetLabelNameFromResourceName("name.subname")).To(Equal("name-subname"))
	})

	It("Should not shorten label name if it fits", func() {
		Expect(GetLabelNameFromResourceName("name")).To(Equal("name"))
		Expect(GetLabelNameFromResourceName(word63)).To(Equal(word63))
	})

	It("Should shorten to long label correctly", func() {
		word64 := util.RandAlphaNum(64)
		result := GetLabelNameFromResourceName(word64)
		Expect(len(result)).To(BeNumerically("<=", 63))

		shortenedWithoutHash := word64[0 : 63-13]
		Expect(result).To(HavePrefix(shortenedWithoutHash))
	})

})
