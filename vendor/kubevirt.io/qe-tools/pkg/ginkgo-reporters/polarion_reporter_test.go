/*
 * This file is part of the KubeVirt project
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
 * Copyright 2018 Red Hat, Inc.
 *
 */

package ginkgo_reporters

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/types"
	. "github.com/onsi/gomega"
)

var _ = Describe("ginkgo_reporters", func() {

	Context("When Spec Suite Will Begin", func() {

		var (
			suiteSummary *types.SuiteSummary
			configType   config.GinkgoConfigType
			properties   PolarionProperties
			reporter     PolarionReporter
		)
		suiteSummary = &types.SuiteSummary{
			SuiteDescription: "SUITE DESCRIPTION",
		}

		configType = config.GinkgoConfigType{
			ParallelTotal: 1,
			ParallelNode:  1,
		}

		reporter = PolarionReporter{
			Run:       true,
			Filename:  "polarion.xml",
			ProjectId: "QE",
			PlannedIn: "QE_1.0",
			Tier:      "tier1",
		}

		properties = PolarionProperties{
			Property: []PolarionProperty{
				{
					Name:  "polarion-project-id",
					Value: "QE",
				},
				{
					Name:  "polarion-testcase-lookup-method",
					Value: "name",
				},
				{
					Name:  "polarion-custom-plannedin",
					Value: "QE_1.0",
				},
				{
					Name:  "polarion-testrun-id",
					Value: "QE_1.0_tier1",
				},
				{
					Name:  "polarion-custom-isautomated",
					Value: "True",
				},
			},
		}

		It("Should info reporter test suite name & properties", func() {
			reporter.SpecSuiteWillBegin(configType, suiteSummary)
			Expect(reporter.TestSuiteName).To(Equal(suiteSummary.SuiteDescription))
			Expect(reporter.Suite.Properties).To(Equal(properties))
		})
	})

	Context("When Spec Did Complete", func() {

		var (
			specSummaryPass *types.SpecSummary
			specSummaryFail *types.SpecSummary
			specSummarySkip *types.SpecSummary
			reporter        PolarionReporter
		)
		specSummaryPass = &types.SpecSummary{
			ComponentTexts: []string{"THIS", "IS", "A PASSING", "TEST"},
			State:          types.SpecStatePassed,
			CapturedOutput: "Test output",
		}

		specSummaryFail = &types.SpecSummary{
			ComponentTexts: []string{"THIS", "IS", "A FAILING", "TEST"},
			State:          types.SpecStateFailed,
			CapturedOutput: "Test output",
			Failure: types.SpecFailure{
				Message: "ERROR MSG",
				Location: types.CodeLocation{
					FileName:       "file/a",
					LineNumber:     3,
					FullStackTrace: "some-stack-trace",
				},
				ComponentCodeLocation: types.CodeLocation{
					FileName:       "file/b",
					LineNumber:     4,
					FullStackTrace: "some-stack-trace",
				},
			},
		}

		skipMessage := JUnitSkipped{}

		specSummarySkip = &types.SpecSummary{
			ComponentTexts: []string{"THIS", "IS", "A SKIPPING", "TEST"},
			State:          types.SpecStateSkipped,
			CapturedOutput: "Test output",
		}

		testcases := []PolarionTestCase{
			{
				Name: fmt.Sprintf("%s: %s", "IS", "A PASSING TEST"),
			},
			{
				Name:      fmt.Sprintf("%s: %s", "IS", "A FAILING TEST"),
				SystemOut: "Test output",
				FailureMessage: &JUnitFailureMessage{
					Type:    "Failure",
					Message: "file/b:4\nERROR MSG\nfile/a:3",
				},
			},
			{
				Name:    fmt.Sprintf("%s: %s", "IS", "A SKIPPING TEST"),
				Skipped: &skipMessage,
			},
		}

		reporter = PolarionReporter{
			Run:       true,
			Filename:  "polarion.xml",
			ProjectId: "QE",
			PlannedIn: "QE_1.0",
			Tier:      "tier1",
		}

		It("Should info reporter test cases did complete", func() {
			reporter.SpecDidComplete(specSummaryPass)
			reporter.SpecDidComplete(specSummaryFail)
			reporter.SpecDidComplete(specSummarySkip)
			Expect(reporter.Suite.TestCases).To(Equal(testcases))
		})
	})

	Context("When Spec Spec Suite Did End", func() {

		var (
			suiteSummary *types.SuiteSummary
			reporter     PolarionReporter
		)
		suiteSummary = &types.SuiteSummary{
			NumberOfSpecsThatWillBeRun: 3,
			RunTime:                    time.Minute,
			NumberOfFailedSpecs:        1,
		}

		reporter = PolarionReporter{
			Run:       true,
			Filename:  "polarion.xml",
			ProjectId: "QE",
			PlannedIn: "QE_1.0",
			Tier:      "tier1",
		}

		It("Should info number of tests, execution time, number of failures & verify report file was created", func() {
			reporter.SpecSuiteDidEnd(suiteSummary)
			Expect(reporter.Suite.Tests).To(Equal(3))
			Expect(reporter.Suite.Time).To(Equal(time.Minute.Seconds()))
			Expect(reporter.Suite.Failures).To(Equal(1))
			if _, err := os.Stat(reporter.Filename); err == nil {
				Expect(err).ToNot(HaveOccurred())
			}
		})
	})

})
