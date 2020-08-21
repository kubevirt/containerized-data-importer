package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

// MakeEmptyCDIConfigSpec creates cdi config manifest
func MakeEmptyCDIConfigSpec(name string) *cdiv1.CDIConfig {
	return &cdiv1.CDIConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CDIConfig",
			APIVersion: "cdi.kubevirt.io/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: "",
			},
		},
	}
}

// MakeEmptyCDICR creates CDI CustomResouce manifest
func MakeEmptyCDICR() *cdiv1.CDI {
	return &cdiv1.CDI{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CDI",
			APIVersion: "cdis.cdi.kubevirt.io",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cdi",
		},
	}
}

// IgnoreNotFound returns nil if the error is a NotFound error.
// We generally want to ignore (not requeue) NotFound errors, since we'll get a reconciliation request once the
// object exists, and requeuing in the meantime won't help.
func IgnoreNotFound(err error) error {
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// IgnoreIsNoMatchError returns nil if the error is a IsNoMatchError.
// We will want to ignore this error for optional CRDs, if it is not found, just ignore it.
func IgnoreIsNoMatchError(err error) error {
	if meta.IsNoMatchError(err) {
		return nil
	}
	return err
}
