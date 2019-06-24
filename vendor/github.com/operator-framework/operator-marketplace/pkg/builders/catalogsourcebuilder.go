package builders

import (
	olm "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CatalogSourceBuilder builds a new CatalogSource object.
type CatalogSourceBuilder struct {
	cs olm.CatalogSource
}

// CatalogSource returns a CatalogSource object.
func (b *CatalogSourceBuilder) CatalogSource() *olm.CatalogSource {
	return &b.cs
}

// WithTypeMeta sets basic TypeMeta.
func (b *CatalogSourceBuilder) WithTypeMeta() *CatalogSourceBuilder {
	b.cs.TypeMeta = metav1.TypeMeta{
		Kind:       olm.CatalogSourceKind,
		APIVersion: olm.CatalogSourceCRDAPIVersion,
	}
	return b
}

// WithMeta sets basic TypeMeta and ObjectMeta.
func (b *CatalogSourceBuilder) WithMeta(name, namespace string) *CatalogSourceBuilder {
	b.WithTypeMeta()
	objectMeta := b.cs.GetObjectMeta()
	if objectMeta == nil {
		b.cs.ObjectMeta = metav1.ObjectMeta{}
	}
	b.cs.SetName(name)
	b.cs.SetNamespace(namespace)
	return b
}

// WithOLMLabels adds "olm-visibility", "openshift-marketplace" and and all
// label(s) associated with the CatalogSource object specified in cscLabels.
func (b *CatalogSourceBuilder) WithOLMLabels(cscLabels map[string]string) *CatalogSourceBuilder {
	labels := map[string]string{
		"olm-visibility":        "hidden",
		"openshift-marketplace": "true",
	}

	for key, value := range cscLabels {
		labels[key] = value
	}

	for key, value := range b.cs.GetLabels() {
		labels[key] = value
	}

	b.WithTypeMeta()
	objectMeta := b.cs.GetObjectMeta()
	if objectMeta == nil {
		b.cs.ObjectMeta = metav1.ObjectMeta{}
	}
	b.cs.SetLabels(labels)
	return b
}

// WithOwnerLabel sets the owner label of the CatalogSource object to the given owner.
func (b *CatalogSourceBuilder) WithOwnerLabel(name, namespace string) *CatalogSourceBuilder {
	labels := map[string]string{
		OwnerNameLabel:      name,
		OwnerNamespaceLabel: namespace,
	}

	for key, value := range b.cs.GetLabels() {
		labels[key] = value
	}

	b.cs.SetLabels(labels)
	return b
}

// WithSpec sets Spec with input data.
func (b *CatalogSourceBuilder) WithSpec(csType olm.SourceType, address, displayName, publisher string) *CatalogSourceBuilder {
	b.cs.Spec = olm.CatalogSourceSpec{
		SourceType:  csType,
		Address:     address,
		DisplayName: displayName,
		Publisher:   publisher,
	}
	return b
}
