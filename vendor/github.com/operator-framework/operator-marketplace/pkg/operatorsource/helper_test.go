package operatorsource_test

import (
	"fmt"

	"github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/operator-framework/operator-marketplace/pkg/apis"
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/shared"
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/operator-framework/operator-marketplace/pkg/apis/operators/v2"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func helperGetContextLogger() *log.Entry {
	return log.NewEntry(log.New())
}

func helperNewOperatorSourceWithEndpoint(namespace, name, endpointType string) *v1.OperatorSource {
	return &v1.OperatorSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s",
				v1.SchemeGroupVersion.Group, v1.SchemeGroupVersion.Version),
			Kind: v1.OperatorSourceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},

		Spec: v1.OperatorSourceSpec{
			Type:     endpointType,
			Endpoint: "http://localhost:5000/cnr",
		},
	}
}

func helperNewOperatorSourceWithPhase(namespace, name, phase string) *v1.OperatorSource {
	return &v1.OperatorSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s",
				v1.SchemeGroupVersion.Group, v1.SchemeGroupVersion.Version),
			Kind: v1.OperatorSourceKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},

		Spec: v1.OperatorSourceSpec{
			Type:     "appregistry",
			Endpoint: "http://localhost:5000/cnr",
		},

		Status: v1.OperatorSourceStatus{
			CurrentPhase: shared.ObjectPhase{
				Phase: shared.Phase{
					Name: phase,
				},
			},
		},
	}
}

func helperNewCatalogSourceConfig(namespace, name string) *v2.CatalogSourceConfig {
	return &v2.CatalogSourceConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fmt.Sprintf("%s/%s",
				v2.SchemeGroupVersion.Group, v2.SchemeGroupVersion.Version),
			Kind: v2.CatalogSourceConfigKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func helperNewCatalogSourceConfigWithLabels(namespace, name string, opsrcLabels map[string]string) *v2.CatalogSourceConfig {
	csc := helperNewCatalogSourceConfig(namespace, name)

	// This is the default label that should get added to CatalogSourceConfig.
	labels := map[string]string{
		"opsrc-datastore": "true",
	}

	for key, value := range opsrcLabels {
		labels[key] = value
	}

	csc.SetLabels(labels)

	return csc
}

func NewFakeClient() client.Client {
	scheme := runtime.NewScheme()
	apis.AddToScheme(scheme)
	return fake.NewFakeClientWithScheme(scheme)
}

func NewFakeClientWithCSC(csc *v2.CatalogSourceConfig) client.Client {
	objs := []runtime.Object{
		csc,
	}

	scheme := runtime.NewScheme()
	apis.AddToScheme(scheme)

	return fake.NewFakeClientWithScheme(scheme, objs...)
}

func NewFakeClientWithOpsrc(opsrc *v1.OperatorSource) client.Client {
	scheme := runtime.NewScheme()
	apis.AddToScheme(scheme)

	objs := []runtime.Object{
		opsrc,
	}

	return fake.NewFakeClientWithScheme(scheme, objs...)
}

func NewFakeClientWithChildResources(deployment *appsv1.Deployment, service *corev1.Service, cs *v1alpha1.CatalogSource) client.Client {
	objs := []runtime.Object{
		deployment,
	}

	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(v1.SchemeGroupVersion, deployment, service, cs)
	apis.AddToScheme(scheme)

	return fake.NewFakeClientWithScheme(scheme, objs...)
}
