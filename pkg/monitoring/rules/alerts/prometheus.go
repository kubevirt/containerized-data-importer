package alerts

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/machadovilaca/operator-observability/pkg/operatorrules"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	prometheusRunbookAnnotationKey = "runbook_url"
	defaultRunbookURLTemplate      = "https://kubevirt.io/monitoring/runbooks/%s"
	runbookURLTemplateEnv          = "RUNBOOK_URL_TEMPLATE"

	severityAlertLabelKey        = "severity"
	operatorHealthImpactLabelKey = "operator_health_impact"
	partOfAlertLabelKey          = "kubernetes_operator_part_of"
	componentAlertLabelKey       = "kubernetes_operator_component"
	partOfAlertLabelValue        = "kubevirt"

	componentAlertLabelValue = common.CDILabelValue
)

// Register sets up alert rules in the given namespace.
func Register(namespace string) error {
	alerts := [][]promv1.Rule{
		operatorAlerts,
	}

	runbookURLTemplate := GetRunbookURLTemplate()
	for _, alertGroup := range alerts {
		for _, alert := range alertGroup {
			alert.Labels[partOfAlertLabelKey] = partOfAlertLabelValue
			alert.Labels[componentAlertLabelKey] = componentAlertLabelValue
			alert.Annotations[prometheusRunbookAnnotationKey] = fmt.Sprintf(runbookURLTemplate, alert.Alert)
		}
	}

	return operatorrules.RegisterAlerts(alerts...)
}

// GetRunbookURLTemplate fetches or defaults the runbook URL template.
func GetRunbookURLTemplate() string {
	runbookURLTemplate, exists := os.LookupEnv(runbookURLTemplateEnv)
	if !exists {
		runbookURLTemplate = defaultRunbookURLTemplate
	}

	if strings.Count(runbookURLTemplate, "%s") != 1 {
		panic(errors.New("runbook URL template must have exactly 1 %s substring"))
	}

	return runbookURLTemplate
}
