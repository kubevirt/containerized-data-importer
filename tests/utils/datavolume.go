package utils

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8sv1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
)

const (
	dataVolumePollInterval = 3 * time.Second
	dataVolumeCreateTime   = 270 * time.Second
	dataVolumeDeleteTime   = 270 * time.Second
	dataVolumePhaseTime    = 270 * time.Second
)

const (
	// TinyCoreIsoURL provides a test url for the tineyCore iso image
	TinyCoreIsoURL = "http://cdi-file-host.%s/tinyCore.iso"
	// TinyCoreQcow2URL provides a test url for the tineyCore qcow2 image
	TinyCoreQcow2URL = "http://cdi-file-host.%s/tinyCore.qcow2"
	//TinyCoreIsoRegistryURL provides a test url for the tinycore.qcow2 image wrapped in docker container
	TinyCoreIsoRegistryURL = "docker://cdi-docker-registry-host.%s/tinycoreqcow2"
	//TinyCoreIsoRegistryProxyURL provides a test url for the tinycore.qcow2 image wrapped in docker container available through rate-limiting proxy
	TinyCoreIsoRegistryProxyURL = "docker://cdi-file-host.%s:83/tinycoreqcow2"
	// HTTPSTinyCoreIsoURL provides a test (https) url for the tineyCore iso image
	HTTPSTinyCoreIsoURL = "https://cdi-file-host.%s/tinyCore.iso"
	// HTTPSTinyCoreQcow2URL provides a test (https) url for the tineyCore qcow2 image
	HTTPSTinyCoreQcow2URL = "https://cdi-file-host.%s/tinyCore.qcow2"
	// TinyCoreQcow2URLRateLimit provides a test url for the tineyCore qcow2 image via rate-limiting proxy
	TinyCoreQcow2URLRateLimit = "http://cdi-file-host.%s:82/tinyCore.qcow2"
	// InvalidQcowImagesURL provides a test url for invalid qcow images
	InvalidQcowImagesURL = "http://cdi-file-host.%s/invalid_qcow_images/"
	// LargeVirtualDiskQcow provides a test url for a cirros image with a large virtual size, in qcow2 format
	LargeVirtualDiskQcow = "http://cdi-file-host.%s/cirros-large-virtual-size.qcow2"
	// LargeVirtualDiskXz provides a test url for a cirros image with a large virtual size, in RAW format, XZ-compressed
	LargeVirtualDiskXz = "http://cdi-file-host.%s/cirros-large-virtual-size.raw.xz"
	// LargePhysicalDiskQcow provides a test url for a cirros image with a large physical size, in qcow2 format
	LargePhysicalDiskQcow = "http://cdi-file-host.%s/cirros-large-physical-size.qcow2"
	// LargePhysicalDiskXz provides a test url for a cirros image with a large physical size, in RAW format, XZ-compressed
	LargePhysicalDiskXz = "http://cdi-file-host.%s/cirros-large-physical-size.raw.xz"
	// TarArchiveURL provides a test url for a tar achive file
	TarArchiveURL = "http://cdi-file-host.%s/archive.tar"
	// CirrosURL provides the standard cirros image qcow image
	CirrosURL = "http://cdi-file-host.%s/cirros-qcow2.img"
	// ImageioURL provides URL of oVirt engine hosting imageio
	ImageioURL = "https://imageio.%s:12346/ovirt-engine/api"
	// VcenterURL provides URL of vCenter/ESX simulator
	VcenterURL = "https://vcenter.%s:8989/sdk"
)

// CreateDataVolumeFromDefinition is used by tests to create a testable Data Volume
func CreateDataVolumeFromDefinition(clientSet *cdiclientset.Clientset, namespace string, def *cdiv1.DataVolume) (*cdiv1.DataVolume, error) {
	var dataVolume *cdiv1.DataVolume
	err := wait.PollImmediate(dataVolumePollInterval, dataVolumeCreateTime, func() (bool, error) {
		var err error
		dataVolume, err = clientSet.CdiV1beta1().DataVolumes(namespace).Create(context.TODO(), def, metav1.CreateOptions{})
		if err == nil || apierrs.IsAlreadyExists(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return nil, err
	}
	return dataVolume, nil
}

// DeleteDataVolume deletes the DataVolume with the given name
func DeleteDataVolume(clientSet *cdiclientset.Clientset, namespace, name string) error {
	return wait.PollImmediate(dataVolumePollInterval, dataVolumeDeleteTime, func() (bool, error) {
		err := clientSet.CdiV1beta1().DataVolumes(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
		if err == nil || apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
}

// NewCloningDataVolume initializes a DataVolume struct with PVC annotations
func NewCloningDataVolume(dataVolumeName, size string, sourcePvc *k8sv1.PersistentVolumeClaim) *cdiv1.DataVolume {
	return NewDataVolumeForImageCloning(dataVolumeName, size, sourcePvc.Namespace, sourcePvc.Name, sourcePvc.Spec.StorageClassName, sourcePvc.Spec.VolumeMode)
}

// NewDataVolumeWithHTTPImport initializes a DataVolume struct with HTTP annotations
func NewDataVolumeWithHTTPImport(dataVolumeName string, size string, httpURL string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: httpURL,
				},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
					},
				},
			},
		},
	}
}

// NewDataVolumeWithImageioImport initializes a DataVolume struct with Imageio annotations
func NewDataVolumeWithImageioImport(dataVolumeName string, size string, httpURL string, secret string, configMap string, diskID string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolumeName,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				Imageio: &cdiv1.DataVolumeSourceImageIO{
					URL:           httpURL,
					SecretRef:     secret,
					CertConfigMap: configMap,
					DiskID:        diskID,
				},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
					},
				},
			},
		},
	}
}

// NewDataVolumeWithHTTPImportToBlockPV initializes a DataVolume struct with HTTP annotations to import to block PV
func NewDataVolumeWithHTTPImportToBlockPV(dataVolumeName string, size string, httpURL, storageClassName string) *cdiv1.DataVolume {
	volumeMode := corev1.PersistentVolumeMode(corev1.PersistentVolumeBlock)
	dataVolume := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolumeName,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: httpURL,
				},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				VolumeMode:       &volumeMode,
				StorageClassName: &storageClassName,
				AccessModes:      []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
					},
				},
			},
		},
	}
	return dataVolume
}

// NewDataVolumeCloneToBlockPV initializes a DataVolume for block cloning
func NewDataVolumeCloneToBlockPV(dataVolumeName string, size string, srcNamespace, srcName, storageClassName string) *cdiv1.DataVolume {
	volumeMode := corev1.PersistentVolumeMode(corev1.PersistentVolumeBlock)
	dataVolume := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolumeName,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      srcName,
					Namespace: srcNamespace,
				},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				VolumeMode:       &volumeMode,
				StorageClassName: &storageClassName,
				AccessModes:      []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
					},
				},
			},
		},
	}
	return dataVolume
}

// NewDataVolumeForUpload initializes a DataVolume struct with Upload annotations
func NewDataVolumeForUpload(dataVolumeName string, size string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				Upload: &cdiv1.DataVolumeSourceUpload{},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
					},
				},
			},
		},
	}
}

// NewDataVolumeForBlankRawImage initializes a DataVolume struct for creating blank raw image
func NewDataVolumeForBlankRawImage(dataVolumeName, size string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				Blank: &cdiv1.DataVolumeBlankImage{},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
					},
				},
			},
		},
	}
}

// NewDataVolumeForBlankRawImageBlock initializes a DataVolume struct for creating blank raw image for a block device
func NewDataVolumeForBlankRawImageBlock(dataVolumeName, size string, storageClassName string) *cdiv1.DataVolume {
	volumeMode := corev1.PersistentVolumeMode(corev1.PersistentVolumeBlock)
	dataVolume := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolumeName,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				Blank: &cdiv1.DataVolumeBlankImage{},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				VolumeMode:       &volumeMode,
				StorageClassName: &storageClassName,
				AccessModes:      []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
					},
				},
			},
		},
	}
	return dataVolume
}

// NewDataVolumeForImageCloning initializes a DataVolume struct for cloning disk image
func NewDataVolumeForImageCloning(dataVolumeName, size, namespace, pvcName string, storageClassName *string, volumeMode *k8sv1.PersistentVolumeMode) *cdiv1.DataVolume {
	dv := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Namespace: namespace,
					Name:      pvcName,
				},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
					},
				},
			},
		},
	}
	if volumeMode != nil {
		dv.Spec.PVC.VolumeMode = volumeMode
	}
	if storageClassName != nil {
		dv.Spec.PVC.StorageClassName = storageClassName
	}
	return dv
}

// NewDataVolumeWithRegistryImport initializes a DataVolume struct with registry annotations
func NewDataVolumeWithRegistryImport(dataVolumeName string, size string, registryURL string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolumeName,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				Registry: &cdiv1.DataVolumeSourceRegistry{
					URL: registryURL,
				},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
					},
				},
			},
		},
	}
}

// ModifyDataVolumeWithVDDKImportToBlockPV modifies a DataVolume struct (created by NewDataVolumeWithVddkImport) for importing disks from vCenter/ESX to a block PV
func ModifyDataVolumeWithVDDKImportToBlockPV(dataVolume *cdiv1.DataVolume, storageClassName string) *cdiv1.DataVolume {
	volumeMode := corev1.PersistentVolumeMode(corev1.PersistentVolumeBlock)
	dataVolume.Spec.PVC.VolumeMode = &volumeMode
	dataVolume.Spec.PVC.StorageClassName = &storageClassName
	return dataVolume
}

// NewDataVolumeWithVddkImport initializes a DataVolume struct for importing disks from vCenter/ESX
func NewDataVolumeWithVddkImport(dataVolumeName string, size string, backingFile string, secretRef string, thumbprint string, httpURL string, uuid string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolumeName,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				VDDK: &cdiv1.DataVolumeSourceVDDK{
					BackingFile: backingFile,
					SecretRef:   secretRef,
					Thumbprint:  thumbprint,
					URL:         httpURL,
					UUID:        uuid,
				},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
					},
				},
			},
		},
	}
}

// NewDataVolumeWithVddkWarmImport initializes a DataVolume struct for a multi-stage import from vCenter/ESX snapshots
func NewDataVolumeWithVddkWarmImport(dataVolumeName string, size string, backingFile string, secretRef string, thumbprint string, httpURL string, uuid string, currentCheckpoint string, previousCheckpoint string, finalCheckpoint bool) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolumeName,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				VDDK: &cdiv1.DataVolumeSourceVDDK{
					BackingFile: backingFile,
					SecretRef:   secretRef,
					Thumbprint:  thumbprint,
					URL:         httpURL,
					UUID:        uuid,
				},
			},
			FinalCheckpoint: finalCheckpoint,
			Checkpoints: []cdiv1.DataVolumeCheckpoint{
				{Current: previousCheckpoint, Previous: ""},
				{Current: currentCheckpoint, Previous: previousCheckpoint},
			},
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
					},
				},
			},
		},
	}
}

// WaitForDataVolumePhase waits for DV's phase to be in a particular phase (Pending, Bound, or Lost)
func WaitForDataVolumePhase(clientSet *cdiclientset.Clientset, namespace string, phase cdiv1.DataVolumePhase, dataVolumeName string) error {
	return WaitForDataVolumePhaseWithTimeout(clientSet, namespace, phase, dataVolumeName, dataVolumePhaseTime)
}

// WaitForDataVolumePhaseWithTimeout waits for DV's phase to be in a particular phase (Pending, Bound, or Lost) with a specified timeout
func WaitForDataVolumePhaseWithTimeout(clientSet *cdiclientset.Clientset, namespace string, phase cdiv1.DataVolumePhase, dataVolumeName string, timeout time.Duration) error {
	err := wait.PollImmediate(dataVolumePollInterval, timeout, func() (bool, error) {
		dataVolume, err := clientSet.CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
		if err != nil || dataVolume.Status.Phase != phase {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("DataVolume %s not in phase %s within %v", dataVolumeName, phase, timeout)
	}
	return nil
}

// NewDataVolumeWithArchiveContent initializes a DataVolume struct with 'archive' ContentType
func NewDataVolumeWithArchiveContent(dataVolumeName string, size string, httpURL string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolumeName,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: httpURL,
				},
			},
			ContentType: "archive",
			PVC: &k8sv1.PersistentVolumeClaimSpec{
				AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				Resources: k8sv1.ResourceRequirements{
					Requests: k8sv1.ResourceList{
						k8sv1.ResourceName(k8sv1.ResourceStorage): resource.MustParse(size),
					},
				},
			},
		},
	}
}

// PersistentVolumeClaimFromDataVolume creates a PersistentVolumeClaim definition so we can use PersistentVolumeClaim for various operations.
func PersistentVolumeClaimFromDataVolume(datavolume *cdiv1.DataVolume) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      datavolume.Name,
			Namespace: datavolume.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(datavolume, schema.GroupVersionKind{
					Group:   cdiv1.SchemeGroupVersion.Group,
					Version: cdiv1.SchemeGroupVersion.Version,
					Kind:    "DataVolume",
				}),
			},
		},
		Spec: *datavolume.Spec.PVC,
	}
}
