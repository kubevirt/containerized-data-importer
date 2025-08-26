package rules_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rhobs/operator-observability-toolkit/pkg/testutil"

	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules"
)

var _ = Describe("Rules Validation", func() {
	var linter *testutil.Linter

	BeforeEach(func() {
		Expect(rules.SetupRules("")).To(Succeed())
		linter = testutil.New()
	})

	It("Should validate alerts", func() {
		linter.AddCustomAlertValidations(
			testutil.ValidateAlertNameLength,
			testutil.ValidateAlertRunbookURLAnnotation,
			testutil.ValidateAlertHealthImpactLabel,
			testutil.ValidateAlertPartOfAndComponentLabels)

		problems := linter.LintAlerts(rules.ListAlerts())
		Expect(problems).To(BeEmpty())
	})

	It("Should validate recording rules", func() {
		problems := linter.LintRecordingRules(rules.ListRecordingRules())
		Expect(problems).To(BeEmpty())
	})
})
