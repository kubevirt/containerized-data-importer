package tests

import (
	"flag"

	"k8s.io/api/core/v1"

	"k8s.io/client-go/kubernetes"

	clientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"

	. "github.com/onsi/ginkgo"
)

type Framework struct {
	// Base prefix for distinguishing test names.
	BaseName string

	// Standard kube client
	KubeClient *kubernetes.Clientset
	// CDI aware client
	CDIClient *clientset.Clientset

	// Primary generated namespace.
	namespace *v1.Namespace
	// Secondary generated namespace.
	namespaceSecondary *v1.Namespace
}

func NewFramework(baseName string) *Framework {
	f := &Framework{
		BaseName: baseName,
	}

	flag.Parse()

	BeforeEach(f.BeforeEach)
	AfterEach(f.AfterEach)

	return f
}

func (f *Framework) BeforeEach() {
	client, err := GetKubeClient()
	if err != nil {
		Fail("Unable to get KubeClient")
	}
	f.KubeClient = client
	f.CDIClient = GetCDIClientOrDie()

	// Create test namespace
	f.namespace = GenerateNamespace(f.KubeClient, f.BaseName)
}

func (f *Framework) AfterEach() {
	// Clean up namespace
	if f.namespace != nil {
		DestroyNamespace(f.KubeClient, f.namespace)
	}
	if f.namespaceSecondary != nil {
		DestroyNamespace(f.KubeClient, f.namespaceSecondary)
	}
}

func (f *Framework) GenerateSecondaryNamespace() *v1.Namespace {
	f.namespaceSecondary = GenerateNamespace(f.KubeClient, f.BaseName)
	return f.namespaceSecondary
}

func (f *Framework) GetNamespace() *v1.Namespace {
	return f.namespace
}

func (f *Framework) GetSecondaryNamespace() *v1.Namespace {
	return f.namespaceSecondary
}
