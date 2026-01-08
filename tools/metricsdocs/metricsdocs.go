package main

import (
	"fmt"

	"github.com/rhobs/operator-observability-toolkit/pkg/docs"
	om "github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"

	cdiClonerMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-cloner"
	cdiMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-controller"
	cdiImporterMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-importer"
	openstackPopulatorMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/openstack-populator"
	operatorMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/operator-controller"
	ovirtPopulatorMetrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/ovirt-populator"
	"kubevirt.io/containerized-data-importer/pkg/monitoring/rules"
)

const title = `Containerized Data Importer metrics`

func main() {
	err := operatorMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = cdiMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = cdiImporterMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = cdiClonerMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = openstackPopulatorMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	err = ovirtPopulatorMetrics.SetupMetrics()
	if err != nil {
		panic(err)
	}

	if err := rules.SetupRules("test"); err != nil {
		panic(err)
	}

	docsString := docs.BuildMetricsDocs(title, om.ListMetrics(), rules.ListRecordingRules())

	fmt.Print(docsString)
}
