package e2e

import (
	"fmt"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	olm "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/operator-framework/operator-marketplace/pkg/apis"
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/v2"

	"github.com/operator-framework/operator-marketplace/test/testgroups"
	"github.com/operator-framework/operator-sdk/pkg/test"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var coAPINotPresent = true

func TestMain(m *testing.M) {
	test.MainEntry(m)
}

// TestMarketplace is the root function that triggers the set of e2e tests.
func TestMarketplace(t *testing.T) {
	initTestingFramework(t)

	// Run Test Groups
	if coAPINotPresent {
		t.Run("cluster-operator-status-test-group", testgroups.ClusterOperatorTestGroup)
	}
	t.Run("operator-source-test-group", testgroups.OperatorSourceTestGroup)
	t.Run("no-setup-test-group", testgroups.NoSetupTestGroup)
}

// initTestingFramework adds the marketplace OperatorSource and CatalogSourceConfig types as well as the
// olm CatalogSource type to the framework scheme.
func initTestingFramework(t *testing.T) {
	// Add marketplace types to test framework scheme.
	operatorSource := &v1.OperatorSource{
		TypeMeta: metav1.TypeMeta{
			Kind: v1.OperatorSourceKind,
			APIVersion: fmt.Sprintf("%s/%s",
				v1.SchemeGroupVersion.Group, v1.SchemeGroupVersion.Version),
		},
	}
	catalogSourceConfig := &v2.CatalogSourceConfig{
		TypeMeta: metav1.TypeMeta{
			Kind: v2.CatalogSourceConfigKind,
			APIVersion: fmt.Sprintf("%s/%s",
				v2.SchemeGroupVersion.Group, v2.SchemeGroupVersion.Version),
		},
	}
	err := test.AddToFrameworkScheme(apis.AddToScheme, operatorSource)
	if err != nil {
		t.Fatalf("failed to add OperatorSource custom resource scheme to framework: %v", err)
	}
	err = test.AddToFrameworkScheme(apis.AddToScheme, catalogSourceConfig)
	if err != nil {
		t.Fatalf("failed to add CatalogsourceConfig custom resource scheme to framework: %v", err)
	}
	// Add (olm) CatalogSources to framework scheme.
	catalogSource := &olm.CatalogSource{
		TypeMeta: metav1.TypeMeta{
			Kind:       olm.CatalogSourceKind,
			APIVersion: olm.CatalogSourceCRDAPIVersion,
		},
	}
	err = test.AddToFrameworkScheme(olm.AddToScheme, catalogSource)
	if err != nil {
		t.Fatalf("failed to add CatalogSource custom resource scheme to framework: %v", err)
	}
	// Add (configv1) ClusterOperator to framework scheme
	clusterOperator := &configv1.ClusterOperator{
		TypeMeta: metav1.TypeMeta{
			Kind: "ClusterOperator",
			APIVersion: fmt.Sprintf("%s/%s",
				configv1.SchemeGroupVersion.Group, configv1.SchemeGroupVersion.Version),
		},
	}

	// We are assuming that ClusterOperator API will always be added successfully
	// on an OpenShift cluster. This is to allow the tests be run on non-OpenSHift
	// clusters
	err = test.AddToFrameworkScheme(configv1.Install, clusterOperator)
	if err != nil {
		t.Logf("failed to add ClusterOperator custom resource scheme to framework: %v. ClusterOperator test will not run.", err)
		coAPINotPresent = false
	}
}
