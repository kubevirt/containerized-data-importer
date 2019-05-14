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
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"kubevirt.io/qe-tools/pkg/polarion-xml"
)

const (
	ginkgoDescribe = "Describe"
	ginkgoContext  = "Context"
	ginkgoSpecify  = "Specify"
	ginkgoTable    = "DescribeTable"
	ginkgoWhen     = "When"
	ginkgoBy       = "By"
	ginkgoIt       = "It"
)

type ginkgoBlock struct {
	content     []string
	rparenPos   []token.Pos
	steps       []string
	stepContext []token.Pos
	funcBlocks  []funcBlock
}

type funcBlock struct {
	steps []string
	name  string
}

func addCustomField(customFields *polarion_xml.TestCaseCustomFields, id, content string) {
	customFields.CustomFields = append(
		customFields.CustomFields, polarion_xml.TestCaseCustomField{
			Content: content,
			ID:      id,
		})
}

func addLinkedWorkItem(linkedWorkItems *polarion_xml.TestCaseLinkedWorkItems, id string) {
	linkedWorkItems.LinkedWorkItems = append(
		linkedWorkItems.LinkedWorkItems, polarion_xml.TestCaseLinkedWorkItem{
			ID:   id,
			Role: "verifies",
		})
}

func parseTagsFromTitle(testCase *polarion_xml.TestCase, title string, component string, projectID string) {
	posneg := "positive"
	caselevel := "component"
	criticality := "medium"
	rfeID := ""
	title = strings.Replace(title, "]", ",", -1)
	title = strings.Replace(title, "[", ",", -1)
	attrList := strings.Split(title, ",")

	for i := 0; i < len(attrList); i++ {
		attrList[i] = strings.Trim(attrList[i], " ")
		if strings.Contains(attrList[i], "test_id:") {
			testID := strings.Split(strings.Trim(strings.Split(attrList[i], "test_id:")[1], " "), " ")[0]
			testCase.ID = projectID + "-" + testID

		} else if strings.Contains(attrList[i], "rfe_id:") {
			rfeID = projectID + "-" + strings.Split(strings.Trim(strings.Split(attrList[i], "rfe_id:")[1], " "), " ")[0]
		} else if strings.Contains(attrList[i], "crit:") {
			criticality = strings.Split(strings.Trim(strings.Split(attrList[i], "crit:")[1], " "), " ")[0]

		} else if strings.Contains(attrList[i], "posneg:") {
			posneg = strings.Split(strings.Trim(strings.Split(attrList[i], "posneg:")[1], " "), " ")[0]

		} else if strings.Contains(attrList[i], "level:") {
			caselevel = strings.Split(strings.Trim(strings.Split(attrList[i], "level:")[1], " "), " ")[0]

		} else if strings.Contains(attrList[i], "component:") {
			component = strings.Split(strings.Trim(strings.Split(attrList[i], "component:")[1], " "), " ")[0]
		}
	}

	addCustomField(&testCase.TestCaseCustomFields, "caseimportance", criticality)
	addCustomField(&testCase.TestCaseCustomFields, "caseposneg", posneg)
	addCustomField(&testCase.TestCaseCustomFields, "caselevel", caselevel)
	addLinkedWorkItem(&testCase.TestCaseLinkedWorkItems, rfeID)
	if component != "" {
		addCustomField(&testCase.TestCaseCustomFields, "casecomponent", component)
	}
}

func addTestStep(testCaseSteps *polarion_xml.TestCaseSteps, content string, prepend bool) {
	testCaseStep := polarion_xml.TestCaseStep{
		StepColumn: []polarion_xml.TestCaseStepColumn{
			{
				Content: content,
				ID:      "step",
			},
			{
				Content: "Succeeded",
				ID:      "expectedResult",
			},
		},
	}
	if prepend {
		testCaseSteps.Steps = append([]polarion_xml.TestCaseStep{testCaseStep}, testCaseSteps.Steps...)
	} else {
		testCaseSteps.Steps = append(testCaseSteps.Steps, testCaseStep)
	}
}

func parseFunc(block *funcBlock, funcBody *ast.BlockStmt) {
	ast.Inspect(funcBody, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			ident, ok := x.Fun.(*ast.Ident)
			if !ok {
				return false
			}

			if ident.Name != ginkgoBy {
				return false
			} else {
				lit, ok := x.Args[0].(*ast.BasicLit)
				if !ok {
					return false
				}
				block.steps = append(block.steps, strings.Trim(lit.Value, "\""))
			}
		}
		return true
	})
}

func parseIt(testcase *polarion_xml.TestCase, block *ginkgoBlock, funcBody *ast.BlockStmt) {
	// add test steps from BeforeEach()
	for i := len(block.stepContext) - 1; i >= 0; i-- {
		if block.stepContext[i] > funcBody.Rbrace {
			if testcase.TestCaseSteps == nil {
				testcase.TestCaseSteps = &polarion_xml.TestCaseSteps{}
			}
			addTestStep(testcase.TestCaseSteps, block.steps[i], true)
		} else {
			block.stepContext = block.stepContext[:len(block.rparenPos)-1]
			block.steps = block.steps[:len(block.content)-1]
		}
	}

	ast.Inspect(funcBody, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			var ident *ast.Ident
			if v, ok := x.Fun.(*ast.Ident); ok {
				ident = v
			} else if v, ok := x.Fun.(*ast.SelectorExpr); ok {
				ident = v.Sel
			} else {
				return false
			}

			for _, b := range block.funcBlocks {
				if b.name == ident.Name {
					for _, step := range b.steps {
						if testcase.TestCaseSteps == nil {
							testcase.TestCaseSteps = &polarion_xml.TestCaseSteps{}
						}
						addTestStep(testcase.TestCaseSteps, step, false)
					}
					return true
				}
			}

			if len(x.Args) < 1 {
				return true
			}

			var content string
			if v, ok := x.Args[0].(*ast.BasicLit); ok {
				content = v.Value
			} else if v, ok := x.Args[0].(*ast.SelectorExpr); ok {
				content = v.Sel.Name
			} else {
				return true
			}
			value := strings.Trim(content, "\"")

			switch ident.Name {
			case ginkgoBy:
				if testcase.TestCaseSteps == nil {
					testcase.TestCaseSteps = &polarion_xml.TestCaseSteps{}
				}
				addTestStep(testcase.TestCaseSteps, value, false)
			}
		}
		return true
	})
}

func parseTable(testcases *polarion_xml.TestCases, block *ginkgoBlock, exprs []ast.Expr, customFields *polarion_xml.TestCaseCustomFields, filename string, component string) {
	lit := exprs[0].(*ast.BasicLit)
	baseName := strings.Trim(lit.Value, "\"")

	funLit := exprs[1].(*ast.FuncLit)
	tempCase := &polarion_xml.TestCase{}
	parseIt(tempCase, block, funLit.Body)

	for _, entry := range exprs[2:] {
		callerExpr := entry.(*ast.CallExpr)
		for i := len(block.rparenPos) - 1; i >= 0; i-- {
			if block.rparenPos[i] > callerExpr.Rparen {
				break
			} else {
				block.rparenPos = block.rparenPos[:len(block.rparenPos)-1]
				block.content = block.content[:len(block.content)-1]
			}
		}

		var content string
		if v, ok := callerExpr.Args[0].(*ast.BasicLit); ok {
			content = v.Value
		} else if v, ok := callerExpr.Args[0].(*ast.SelectorExpr); ok {
			content = v.Sel.Name
		}

		value := strings.Trim(content, "\"")
		title := fmt.Sprintf(
			"%s:%s %s %s",
			block.content[0],
			strings.Join(block.content[1:], " "),
			baseName,
			value,
		)

		var re = regexp.MustCompile(`\[.*?\]`)
		titleNoTags := re.ReplaceAllString(title, `$1`)

		testCase := &polarion_xml.TestCase{
			Title:                polarion_xml.Title{Content: titleNoTags},
			Description:          polarion_xml.Description{Content: title},
			TestCaseCustomFields: *customFields,
			TestCaseSteps:        tempCase.TestCaseSteps,
		}

		parseTagsFromTitle(testCase, title, component, testcases.ProjectID)
		addCustomField(&testCase.TestCaseCustomFields, "caseautomation", "automated")
		addCustomField(&testCase.TestCaseCustomFields, "testtype", "functional")
		addCustomField(&testCase.TestCaseCustomFields, "subtype1", "-")
		addCustomField(&testCase.TestCaseCustomFields, "subtype2", "-")
		addCustomField(&testCase.TestCaseCustomFields, "automation_script", filename)
		addCustomField(&testCase.TestCaseCustomFields, "upstream", "yes")
		testcases.TestCases = append(testcases.TestCases, *testCase)
	}
}

// FillPolarionTestCases parse ginkgo format test and fill polarion test cases struct accordingly
func FillPolarionTestCases(f *ast.File, testCases *polarion_xml.TestCases, commentMap *ast.CommentMap, filename string, component string) error {
	var block *ginkgoBlock

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			ident, ok := x.Lhs[0].(*ast.Ident)
			if !ok {
				return false
			}

			funcDef, ok := x.Rhs[0].(*ast.FuncLit)
			if !ok {
				return false
			}

			funcBlock := &funcBlock{name: ident.Name}
			parseFunc(funcBlock, funcDef.Body)
			block.funcBlocks = append(block.funcBlocks, *funcBlock)
			return false

		case *ast.CallExpr:
			var ident *ast.Ident
			if v, ok := x.Fun.(*ast.Ident); ok {
				ident = v
			} else if v, ok := x.Fun.(*ast.SelectorExpr); ok {
				ident = v.Sel
			} else {
				return false
			}

			if len(x.Args) < 1 {
				return true
			}

			var content string
			if v, ok := x.Args[0].(*ast.BasicLit); ok {
				content = v.Value
			} else if v, ok := x.Args[0].(*ast.SelectorExpr); ok {
				content = v.Sel.Name
			} else {
				return true
			}
			value := strings.Trim(content, "\"")

			switch ident.Name {
			case ginkgoDescribe, ginkgoContext, ginkgoWhen:
				if block == nil {
					block = &ginkgoBlock{
						content:   []string{value},
						rparenPos: []token.Pos{x.Rparen},
					}
				} else {
					for i := len(block.rparenPos) - 1; i >= 0; i-- {
						if block.rparenPos[i] > x.Rparen {
							block.rparenPos = append(block.rparenPos, x.Rparen)
							block.content = append(block.content, value)
							break
						} else {
							block.rparenPos = block.rparenPos[:len(block.rparenPos)-1]
							block.content = block.content[:len(block.content)-1]
						}
					}
				}
			case ginkgoBy:
				block.steps = append(block.steps, value)
				block.stepContext = append(block.stepContext, block.rparenPos[len(block.rparenPos)-1])
			case ginkgoTable:
				customFields := polarion_xml.TestCaseCustomFields{}
				parseTable(testCases, block, x.Args, &customFields, filename, component)
				return false
			case ginkgoIt, ginkgoSpecify:
				for i := len(block.rparenPos) - 1; i >= 0; i-- {
					if block.rparenPos[i] > x.Rparen {
						break
					} else {
						block.rparenPos = block.rparenPos[:len(block.rparenPos)-1]
						block.content = block.content[:len(block.content)-1]
					}
				}
				title := fmt.Sprintf("%s: %s", block.content[0], value)
				if len(block.content[1:]) > 0 {
					title = fmt.Sprintf("%s: %s %s", block.content[0], strings.Join(block.content[1:], " "), value)
				}

				var re = regexp.MustCompile(`\[.*?\]`)
				titleNoTags := re.ReplaceAllString(title, `$1`)

				testCase := &polarion_xml.TestCase{
					Title:       polarion_xml.Title{Content: titleNoTags},
					Description: polarion_xml.Description{Content: title},
				}

				customFields := polarion_xml.TestCaseCustomFields{}
				addCustomField(&customFields, "caseautomation", "automated")
				addCustomField(&customFields, "testtype", "functional")
				addCustomField(&customFields, "subtype1", "-")
				addCustomField(&customFields, "subtype2", "-")
				addCustomField(&customFields, "automation_script", filename)
				addCustomField(&customFields, "upstream", "yes")

				testCase.TestCaseCustomFields = customFields
				funLit := x.Args[1].(*ast.FuncLit)
				parseIt(testCase, block, funLit.Body)
				parseTagsFromTitle(testCase, title, component, testCases.ProjectID)
				testCases.TestCases = append(testCases.TestCases, *testCase)
				return false
			}
		}
		return true
	})
	return nil
}

func main() {
	// parse input flags
	testsDir := flag.String("tests-dir", ".", "Directory with tests files")
	outputFile := flag.String("output-file", "polarion.xml", "Generated polarion test cases")
	polarionProjectID := flag.String("project-id", "", "Set the Polarion project ID")
	dryRun := flag.String("dry-run", "false", "Dry-run property")
	component := flag.String("component", "", "Component name")
	flag.Parse()

	// collect all test files from the directory
	var files []string
	err := filepath.Walk(*testsDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if !strings.Contains(info.Name(), "_test") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		panic(err)
	}

	// parse all test files and fill polarion test cases
	var testCases = &polarion_xml.TestCases{
		ProjectID: *polarionProjectID,
		Properties: polarion_xml.PolarionProperties{
			Property: []polarion_xml.PolarionProperty{
				{
					Name:  "lookup-method",
					Value: "id",
				},
				{
					Name:  "dry-run",
					Value: *dryRun,
				},
			},
		},
	}
	for _, file := range files {
		// Create the AST by parsing src
		fset := token.NewFileSet() // positions are relative to fset
		f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			panic(err)
		}
		// Create comment map
		cmap := ast.NewCommentMap(fset, f, f.Comments)

		// fill polarion test cases struct
		pathToFile := strings.Split(file, "/")
		filenameShort := pathToFile[len(pathToFile)-1]
		FillPolarionTestCases(f, testCases, &cmap, filenameShort, *component)
	}

	// generate polarion test cases XML file
	polarion_xml.GeneratePolarionXmlFile(*outputFile, testCases)
}
