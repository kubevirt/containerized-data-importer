package recordingrules

import (
	"fmt"

	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
	"github.com/rhobs/operator-observability-toolkit/pkg/operatorrules"

	"k8s.io/apimachinery/pkg/util/intstr"
)

func operatorRecordingRules(namespace string) []operatorrules.RecordingRule {
	return []operatorrules.RecordingRule{
		{
			MetricsOpts: operatormetrics.MetricOpts{
				Name: "kubevirt_cdi_operator_up",
				Help: "[Deprecated] CDI operator status",
			},
			MetricType: operatormetrics.GaugeType,
			Expr: intstr.FromString(
				fmt.Sprintf("sum(up{namespace='%s', pod=~'cdi-operator-.*'} or vector(0))", namespace),
			),
		},
		{
			MetricsOpts: operatormetrics.MetricOpts{
				Name: "cluster:kubevirt_cdi_operator_up:sum",
				Help: "The number of CDI pods that are up",
			},
			MetricType: operatormetrics.GaugeType,
			Expr: intstr.FromString(
				fmt.Sprintf("sum(up{namespace='%s', pod=~'cdi-operator-.*'} or vector(0))", namespace),
			),
		},
	}
}
