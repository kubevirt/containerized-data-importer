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

package main

import (
	"go/ast"
	"go/parser"
	"go/token"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"kubevirt.io/qe-tools/pkg/polarion-xml"
)

var _ = Describe("Polarion Test Cases Generator", func() {
	var testCases *polarion_xml.TestCases
	var testSrc = `
package test

var _ = Describe("[component:Storage][vendor:cnv-qe.redhat.com][crit:medium]Test Case Generator", func() {
	BeforeEach(func() {
		By("Before each test")
	})

	testNameFunc := func(name string) {
		By("Print the name")
		fmt.Println(name)
	}

	testFunc1 := func() int {
		By("Return 1")
		return 1
	}

	table.DescribeTable("[rfe_id:1][posneg:negative][vendor:cnv-qe.redhat.com][level:integration]table test", func() {
		By("Testing the table")
		testNameFunc("table")
		testFunc1()

	},
		table.Entry("[test_id:1]entry 1"),
		table.Entry("[component:Virt]entry 2"),
	)
	
	Context("[posneg:negative][crit:low][vendor:cnv-qe.redhat.com]test context", func() {
	    It("[test_id:3]test it 1", func() {
			By("Testing it 1")
			testFunc1()
			By("Testing it 1")
		})
	})
	
	It("[test_id:4][rfe_id:5][crit:high][vendor:cnv-qe.redhat.com][level:system]test it 2", func() {
		testFunc1()
		By("Testing it 2")
		By("Testing it 2")
	})
})
`

	BeforeEach(func() {
		fset := token.NewFileSet() // positions are relative to fset
		f, err := parser.ParseFile(fset, "", testSrc, parser.ParseComments)
		Expect(err).ToNot(HaveOccurred())

		cmap := ast.NewCommentMap(fset, f, f.Comments)

		testCases = &polarion_xml.TestCases{}
		FillPolarionTestCases(f, testCases, &cmap, "myscript.go", "")

		Expect(len(testCases.TestCases)).To(Equal(4))
	})

	It("should generate correct titles", func() {
		generateNames := map[int]string{
			0: "Test Case Generator: table test entry 1",
			1: "Test Case Generator: table test entry 2",
			2: "Test Case Generator: test context test it 1",
			3: "Test Case Generator: test it 2",
		}

		for i := range generateNames {
			Expect(testCases.TestCases[i].Title.Content).To(Equal(generateNames[i]))
		}
	})

	It("should generate correct test ID", func() {
		generatedTestIDs := map[int]string{
			0: "-1",
			2: "-3",
			3: "-4",
		}

		for i := range generatedTestIDs {
			Expect(testCases.TestCases[i].ID).To(Equal(generatedTestIDs[i]))
		}
	})

	It("should generate correct steps", func() {
		generatedSteps := map[int]polarion_xml.TestCaseSteps{
			0: {
				Steps: []polarion_xml.TestCaseStep{
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Before each test", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Testing the table", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Print the name", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Return 1", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
				},
			},
			1: {
				Steps: []polarion_xml.TestCaseStep{
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Before each test", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Testing the table", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Print the name", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Return 1", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
				},
			},
			2: {
				Steps: []polarion_xml.TestCaseStep{
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Before each test", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Testing it 1", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Return 1", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Testing it 1", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
				},
			},
			3: {
				Steps: []polarion_xml.TestCaseStep{
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Before each test", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Return 1", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Testing it 2", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
					{
						StepColumn: []polarion_xml.TestCaseStepColumn{
							{Content: "Testing it 2", ID: "step"},
							{Content: "Succeeded", ID: "expectedResult"},
						},
					},
				},
			},
		}

		for i := range generatedSteps {
			for j, step := range testCases.TestCases[i].TestCaseSteps.Steps {
				Expect(step.StepColumn[0].Content).To(Equal(generatedSteps[i].Steps[j].StepColumn[0].Content))
				Expect(step.StepColumn[1].Content).To(Equal(generatedSteps[i].Steps[j].StepColumn[1].Content))
			}
		}
	})

	It("should generate correct custom fields", func() {
		generatedCustomFields := map[int]polarion_xml.TestCaseCustomFields{
			0: {
				CustomFields: []polarion_xml.TestCaseCustomField{
					{Content: "medium", ID: "caseimportance"},
					{Content: "negative", ID: "caseposneg"},
					{Content: "integration", ID: "caselevel"},
					{Content: "Storage", ID: "casecomponent"},
					{Content: "automated", ID: "caseautomation"},
					{Content: "functional", ID: "testtype"},
					{Content: "-", ID: "subtype1"},
					{Content: "-", ID: "subtype2"},
					{Content: "myscript.go", ID: "automation_script"},
					{Content: "yes", ID: "upstream"},
				},
			},
			1: {
				CustomFields: []polarion_xml.TestCaseCustomField{
					{Content: "medium", ID: "caseimportance"},
					{Content: "negative", ID: "caseposneg"},
					{Content: "integration", ID: "caselevel"},
					{Content: "Virt", ID: "casecomponent"},
					{Content: "automated", ID: "caseautomation"},
					{Content: "functional", ID: "testtype"},
					{Content: "-", ID: "subtype1"},
					{Content: "-", ID: "subtype2"},
					{Content: "myscript.go", ID: "automation_script"},
					{Content: "yes", ID: "upstream"},
				},
			},
			2: {
				CustomFields: []polarion_xml.TestCaseCustomField{
					{Content: "automated", ID: "caseautomation"},
					{Content: "functional", ID: "testtype"},
					{Content: "-", ID: "subtype1"},
					{Content: "-", ID: "subtype2"},
					{Content: "myscript.go", ID: "automation_script"},
					{Content: "yes", ID: "upstream"},
					{Content: "low", ID: "caseimportance"},
					{Content: "negative", ID: "caseposneg"},
					{Content: "component", ID: "caselevel"},
					{Content: "Storage", ID: "casecomponent"},
				},
			},
			3: {
				CustomFields: []polarion_xml.TestCaseCustomField{
					{Content: "automated", ID: "caseautomation"},
					{Content: "functional", ID: "testtype"},
					{Content: "-", ID: "subtype1"},
					{Content: "-", ID: "subtype2"},
					{Content: "myscript.go", ID: "automation_script"},
					{Content: "yes", ID: "upstream"},
					{Content: "high", ID: "caseimportance"},
					{Content: "positive", ID: "caseposneg"},
					{Content: "system", ID: "caselevel"},
					{Content: "Storage", ID: "casecomponent"},
				},
			},
		}
		for i := range generatedCustomFields {
			for j, customField := range testCases.TestCases[i].TestCaseCustomFields.CustomFields {
				Expect(customField.Content).To(Equal(generatedCustomFields[i].CustomFields[j].Content))
				Expect(customField.ID).To(Equal(generatedCustomFields[i].CustomFields[j].ID))
			}
		}
	})
})
