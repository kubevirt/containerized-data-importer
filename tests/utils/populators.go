package utils

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// NewVolumeImportSourceWithVddkWarmImport initializes a VolumeImportSource for a multi-stage import from vCenter/ESX snapshots
func NewVolumeImportSourceWithVddkWarmImport(name, pvcName, backingFile, secretRef, thumbprint, httpURL, uuid, currentCheckpoint, previousCheckpoint string, finalCheckpoint bool) *cdiv1.VolumeImportSource {
	return &cdiv1.VolumeImportSource{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: cdiv1.VolumeImportSourceSpec{
			Source: &cdiv1.ImportSourceType{
				VDDK: &cdiv1.DataVolumeSourceVDDK{
					BackingFile: backingFile,
					SecretRef:   secretRef,
					Thumbprint:  thumbprint,
					URL:         httpURL,
					UUID:        uuid,
				},
			},
			TargetClaim:     &pvcName,
			FinalCheckpoint: &finalCheckpoint,
			Checkpoints: []cdiv1.DataVolumeCheckpoint{
				{Current: previousCheckpoint, Previous: ""},
				{Current: currentCheckpoint, Previous: previousCheckpoint},
			},
		},
	}
}
