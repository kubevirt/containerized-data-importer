package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	k8sv1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	dvc "kubevirt.io/containerized-data-importer/pkg/controller/datavolume"
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
	// TinyCoreIsoAuthURL provides a tinyCore ISO from a URL that requires basic authentication
	TinyCoreIsoAuthURL = "http://cdi-file-host.%s:81/tinyCore.iso"
	// HTTPSTinyCoreIsoURL provides a test (https) url for the tineyCore iso image
	HTTPSTinyCoreIsoURL = "https://cdi-file-host.%s/tinyCore.iso"
	// HTTPSTinyCoreQcow2URL provides a test (https) url for the tineyCore qcow2 image
	HTTPSTinyCoreQcow2URL = "https://cdi-file-host.%s/tinyCore.qcow2"
	// HTTPSTinyCoreZstURL provides a test (https) url for the tineyCore zst image
	HTTPSTinyCoreZstURL = "https://cdi-file-host.%s/tinyCore.iso.zst"
	// TinyCoreQcow2URLRateLimit provides a test url for the tineyCore qcow2 image via rate-limiting proxy
	TinyCoreQcow2URLRateLimit = "http://cdi-file-host.%s:82/tinyCore.qcow2"
	// TinyCoreQcow2GzURLRateLimit provides a test url for the tineyCore qcow2.gz image via rate-limiting proxy
	TinyCoreQcow2GzURLRateLimit = "http://cdi-file-host.%s:82/tinyCore.qcow2.gz"
	// HTTPSTinyCoreVmdkURL provides a test url for the tineyCore qcow2 image
	HTTPSTinyCoreVmdkURL = "https://cdi-file-host.%s/tinyCore.vmdk"
	// HTTPSTinyCoreVdiURL provides a test url for the tineyCore qcow2 image
	HTTPSTinyCoreVdiURL = "https://cdi-file-host.%s/tinyCore.vdi"
	// HTTPSTinyCoreVhdURL provides a test url for the tineyCore qcow2 image
	HTTPSTinyCoreVhdURL = "https://cdi-file-host.%s/tinyCore.vhd"
	// HTTPSTinyCoreVhdxURL provides a test url for the tineyCore qcow2 image
	HTTPSTinyCoreVhdxURL = "https://cdi-file-host.%s/tinyCore.vhdx"
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
	// CirrosGCSQCOWURL provides the standard cirros image qcow image for GCS
	CirrosGCSQCOWURL = "http://cdi-file-host.%s/gcs-bucket/cirros-qcow2.img"
	// CirrosGCSRAWURL provides the standard cirros image raw image for GCS
	CirrosGCSRAWURL = "http://cdi-file-host.%s/gcs-bucket/cirros.raw"
	// ImageioURL provides URL of oVirt engine hosting imageio
	ImageioURL = "https://imageio.%s:12346/ovirt-engine/api"
	// ImageioRootURL provides the base path to fakeovirt, for inventory modifications
	ImageioRootURL = "https://imageio.%s:12346"
	// ImageioImageURL provides the base path to imageiotest, for test images
	ImageioImageURL = "https://imageio.%s:12345"
	// VcenterURL provides URL of vCenter/ESX simulator
	VcenterURL = "https://vcenter.%s:8989/sdk"
	// TrustedRegistryURL provides the base path to trusted registry test url for the tinycore.iso image wrapped in docker container
	TrustedRegistryURL = "docker://%s/cdi-func-test-tinycore"
	// TrustedRegistryIS provides the base path to trusted registry test fake imagestream for the tinycore.iso image wrapped in docker container
	TrustedRegistryIS = "%s/cdi-func-test-tinycore"

	// MD5PrefixSize is the number of bytes used by the MD5 constants below
	MD5PrefixSize = int64(100000)
	// TinyCoreMD5 is the MD5 hash of first 100k bytes of tinyCore image
	TinyCoreMD5 = "3710416a680523c7d07538cb1026c60c"
	// TinyCoreTarMD5 is the MD5 hash of first 100k bytes of tinyCore tar image
	TinyCoreTarMD5 = "aec1a39d753b4b7cc81ee02bc625a342"
	// ImageioMD5 is the MD5 hash of first 100k bytes of imageio image
	ImageioMD5 = "91150be031835ccfac458744da57d4f6"
	// VcenterMD5 is the MD5 hash of first 100k bytes of Vcenter image
	VcenterMD5 = "91150be031835ccfac458744da57d4f6"
	// BlankMD5 is the MD5 hash of first 100k bytes of blank image
	BlankMD5 = "0019d23bef56a136a1891211d7007f6f"
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

// CleanupDvPvc Deletes PVC if DV was GCed, otherwise wait for PVC to be gone
func CleanupDvPvc(k8sClient *kubernetes.Clientset, cdiClient *cdiclientset.Clientset, namespace, name string) {
	err := cdiClient.CdiV1beta1().DataVolumes(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if apierrs.IsNotFound(err) {
		// Must have been GCed, delete PVC
		err = DeletePVC(k8sClient, namespace, name)
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		return
	}
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	deleted, err := WaitPVCDeleted(k8sClient, name, namespace, 2*time.Minute)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(deleted).To(gomega.BeTrue())
}

// NewCloningDataVolume initializes a DataVolume struct with PVC annotations
func NewCloningDataVolume(dataVolumeName, size string, sourcePvc *k8sv1.PersistentVolumeClaim) *cdiv1.DataVolume {
	return NewDataVolumeForImageCloning(dataVolumeName, size, sourcePvc.Namespace, sourcePvc.Name, sourcePvc.Spec.StorageClassName, sourcePvc.Spec.VolumeMode)
}

// NewDataVolumeWithSourceRef initializes a DataVolume struct with DataSource SourceRef
func NewDataVolumeWithSourceRef(dataVolumeName string, size, sourceRefNamespace, sourceRefName string) *cdiv1.DataVolume {
	claimSpec := &k8sv1.PersistentVolumeClaimSpec{
		AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
		Resources: k8sv1.ResourceRequirements{
			Requests: k8sv1.ResourceList{
				k8sv1.ResourceStorage: resource.MustParse(size),
			},
		},
	}

	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			SourceRef: &cdiv1.DataVolumeSourceRef{
				Kind:      cdiv1.DataVolumeDataSource,
				Namespace: &sourceRefNamespace,
				Name:      sourceRefName,
			},
			PVC: claimSpec,
		},
	}
}

// NewDataVolumeWithSourceRefAndStorageAPI initializes a DataVolume struct with DataSource SourceRef and storage defaults-inferring API
func NewDataVolumeWithSourceRefAndStorageAPI(dataVolumeName string, size, sourceRefNamespace, sourceRefName string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			SourceRef: &cdiv1.DataVolumeSourceRef{
				Kind:      cdiv1.DataVolumeDataSource,
				Namespace: &sourceRefNamespace,
				Name:      sourceRefName,
			},
			Storage: &cdiv1.StorageSpec{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(size),
					},
				},
			},
		},
	}
}

// NewPvcDataSource initializes a DataSource struct with PVC source
func NewPvcDataSource(dataSourceName, dataSourceNamespace, pvcName, pvcNamespace string) *cdiv1.DataSource {
	return &cdiv1.DataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dataSourceName,
			Namespace: dataSourceNamespace,
		},
		Spec: cdiv1.DataSourceSpec{
			Source: cdiv1.DataSourceSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      pvcName,
					Namespace: pvcNamespace,
				},
			},
		},
	}
}

// NewSnapshotDataSource initializes a DataSource struct with volumesnapshot source
func NewSnapshotDataSource(dataSourceName, dataSourceNamespace, snapshotName, snapshotNamespace string) *cdiv1.DataSource {
	return &cdiv1.DataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dataSourceName,
			Namespace: dataSourceNamespace,
		},
		Spec: cdiv1.DataSourceSpec{
			Source: cdiv1.DataSourceSource{
				Snapshot: &cdiv1.DataVolumeSourceSnapshot{
					Name:      snapshotName,
					Namespace: snapshotNamespace,
				},
			},
		},
	}
}

// NewDataVolumeWithHTTPImport initializes a DataVolume struct with HTTP annotations
func NewDataVolumeWithHTTPImport(dataVolumeName string, size string, httpURL string) *cdiv1.DataVolume {
	claimSpec := &k8sv1.PersistentVolumeClaimSpec{
		AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
		Resources: k8sv1.ResourceRequirements{
			Requests: k8sv1.ResourceList{
				k8sv1.ResourceStorage: resource.MustParse(size),
			},
		},
	}

	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: httpURL,
				},
			},
			PVC: claimSpec,
		},
	}
}

// NewDataVolumeWithHTTPImportAndStorageSpec initializes a DataVolume struct with HTTP annotations
func NewDataVolumeWithHTTPImportAndStorageSpec(dataVolumeName string, size string, httpURL string) *cdiv1.DataVolume {
	storageSpec := &cdiv1.StorageSpec{
		AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
		Resources: k8sv1.ResourceRequirements{
			Requests: k8sv1.ResourceList{
				k8sv1.ResourceStorage: resource.MustParse(size),
			},
		},
	}

	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				HTTP: &cdiv1.DataVolumeSourceHTTP{
					URL: httpURL,
				},
			},
			Storage: storageSpec,
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
			Source: &cdiv1.DataVolumeSource{
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

// NewDataVolumeWithImageioWarmImport initializes a DataVolume struct for a multi-stage import from oVirt snapshots
func NewDataVolumeWithImageioWarmImport(dataVolumeName string, size string, httpURL string, secret string, configMap string, diskID string, checkpoints []cdiv1.DataVolumeCheckpoint, finalCheckpoint bool) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolumeName,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Imageio: &cdiv1.DataVolumeSourceImageIO{
					URL:           httpURL,
					SecretRef:     secret,
					CertConfigMap: configMap,
					DiskID:        diskID,
				},
			},
			FinalCheckpoint: finalCheckpoint,
			Checkpoints:     checkpoints,
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
			Source: &cdiv1.DataVolumeSource{
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

// NewDataVolumeWithExternalPopulation initializes a DataVolume struct meant to be externally populated
func NewDataVolumeWithExternalPopulation(dataVolumeName, size, storageClassName string, volumeMode corev1.PersistentVolumeMode, dataSource *corev1.TypedLocalObjectReference, dataSourceRef *corev1.TypedObjectReference) *cdiv1.DataVolume {
	dataVolume := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolumeName,
		},
		Spec: cdiv1.DataVolumeSpec{
			PVC: &corev1.PersistentVolumeClaimSpec{
				VolumeMode:       &volumeMode,
				StorageClassName: &storageClassName,
				AccessModes:      []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				DataSource:       dataSource,
				DataSourceRef:    dataSourceRef,
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

// NewDataVolumeWithExternalPopulationAndStorageSpec initializes a DataVolume struct meant to be externally populated (with storage spec)
func NewDataVolumeWithExternalPopulationAndStorageSpec(dataVolumeName, size, storageClassName string, volumeMode corev1.PersistentVolumeMode, dataSource *corev1.TypedLocalObjectReference, dataSourceRef *corev1.TypedObjectReference) *cdiv1.DataVolume {
	dataVolume := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolumeName,
		},
		Spec: cdiv1.DataVolumeSpec{
			Storage: &cdiv1.StorageSpec{
				VolumeMode:       &volumeMode,
				StorageClassName: &storageClassName,
				AccessModes:      []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
				DataSource:       dataSource,
				DataSourceRef:    dataSourceRef,
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
			Source: &cdiv1.DataVolumeSource{
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

// NewDataVolumeCloneToBlockPVStorageAPI initializes a DataVolume for block cloning with Storage API
func NewDataVolumeCloneToBlockPVStorageAPI(dataVolumeName string, size string, srcNamespace, srcName, storageClassName string) *cdiv1.DataVolume {
	volumeMode := corev1.PersistentVolumeBlock
	dataVolume := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: dataVolumeName,
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Name:      srcName,
					Namespace: srcNamespace,
				},
			},
			Storage: &cdiv1.StorageSpec{
				VolumeMode:       &volumeMode,
				StorageClassName: &storageClassName,
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
			Source: &cdiv1.DataVolumeSource{
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

// NewDataVolumeForUploadWithStorageAPI initializes a DataVolume struct with Upload annotations
// and using the Storage API instead of the PVC API
func NewDataVolumeForUploadWithStorageAPI(dataVolumeName string, size string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Upload: &cdiv1.DataVolumeSourceUpload{},
			},
			Storage: &cdiv1.StorageSpec{
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
			Source: &cdiv1.DataVolumeSource{
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
			Source: &cdiv1.DataVolumeSource{
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
			Annotations: map[string]string{cc.AnnDeleteAfterCompletion: "false"},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
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

// NewDataVolumeForImageCloningAndStorageSpec initializes a DataVolume struct with spec.storage for cloning disk image
func NewDataVolumeForImageCloningAndStorageSpec(dataVolumeName, size, namespace, pvcName string, storageClassName *string, volumeMode *k8sv1.PersistentVolumeMode) *cdiv1.DataVolume {
	storageSpec := &cdiv1.StorageSpec{
		AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
		Resources: k8sv1.ResourceRequirements{
			Requests: k8sv1.ResourceList{
				k8sv1.ResourceStorage: resource.MustParse(size),
			},
		},
	}

	dv := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Namespace: namespace,
					Name:      pvcName,
				},
			},
			Storage: storageSpec,
		},
	}
	if volumeMode != nil {
		dv.Spec.Storage.VolumeMode = volumeMode
	}
	if storageClassName != nil {
		dv.Spec.Storage.StorageClassName = storageClassName
	}
	return dv
}

// NewDataVolumeForCloningWithEmptySize initializes a DataVolume struct with empty storage size to test the size-detection mechanism when cloning
func NewDataVolumeForCloningWithEmptySize(dataVolumeName, namespace, pvcName string, storageClassName *string, volumeMode *k8sv1.PersistentVolumeMode) *cdiv1.DataVolume {
	dv := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				PVC: &cdiv1.DataVolumeSourcePVC{
					Namespace: namespace,
					Name:      pvcName,
				},
			},
			Storage: &cdiv1.StorageSpec{
				AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
			},
		},
	}
	if volumeMode != nil {
		dv.Spec.Storage.VolumeMode = volumeMode
	}
	if storageClassName != nil {
		dv.Spec.Storage.StorageClassName = storageClassName
	}
	return dv
}

// NewDataVolumeForSnapshotCloning initializes a DataVolume struct for cloning from a volume snapshot
func NewDataVolumeForSnapshotCloning(dataVolumeName, size, namespace, snapshot string, storageClassName *string, volumeMode *k8sv1.PersistentVolumeMode) *cdiv1.DataVolume {
	dv := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Snapshot: &cdiv1.DataVolumeSourceSnapshot{
					Namespace: namespace,
					Name:      snapshot,
				},
			},
			PVC: &corev1.PersistentVolumeClaimSpec{
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

// NewDataVolumeForSnapshotCloningAndStorageSpec initializes a DataVolume struct for cloning from a volume snapshot
func NewDataVolumeForSnapshotCloningAndStorageSpec(dataVolumeName, size, namespace, snapshot string, storageClassName *string, volumeMode *k8sv1.PersistentVolumeMode) *cdiv1.DataVolume {
	dv := &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Snapshot: &cdiv1.DataVolumeSourceSnapshot{
					Namespace: namespace,
					Name:      snapshot,
				},
			},
			Storage: &cdiv1.StorageSpec{
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
		dv.Spec.Storage.VolumeMode = volumeMode
	}
	if storageClassName != nil {
		dv.Spec.Storage.StorageClassName = storageClassName
	}
	return dv
}

// NewDataVolumeWithRegistryImport initializes a DataVolume struct with registry annotations
func NewDataVolumeWithRegistryImport(dataVolumeName string, size string, registryURL string) *cdiv1.DataVolume {
	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				Registry: &cdiv1.DataVolumeSourceRegistry{
					URL: &registryURL,
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

// ModifyDataVolumeWithImportToBlockPV modifies a DataVolume struct for import to a block PV
func ModifyDataVolumeWithImportToBlockPV(dataVolume *cdiv1.DataVolume, storageClassName string) *cdiv1.DataVolume {
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
			Source: &cdiv1.DataVolumeSource{
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
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
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

// NewDataVolumeWithGCSImport initializes a DataVolume struct with GCS HTTP annotations
func NewDataVolumeWithGCSImport(dataVolumeName string, size string, gcsURL string) *cdiv1.DataVolume {
	claimSpec := &k8sv1.PersistentVolumeClaimSpec{
		AccessModes: []k8sv1.PersistentVolumeAccessMode{k8sv1.ReadWriteOnce},
		Resources: k8sv1.ResourceRequirements{
			Requests: k8sv1.ResourceList{
				k8sv1.ResourceStorage: resource.MustParse(size),
			},
		},
	}

	return &cdiv1.DataVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dataVolumeName,
			Annotations: map[string]string{},
		},
		Spec: cdiv1.DataVolumeSpec{
			Source: &cdiv1.DataVolumeSource{
				GCS: &cdiv1.DataVolumeSourceGCS{
					URL: gcsURL,
				},
			},
			PVC: claimSpec,
		},
	}
}

// WaitForDataVolumePhase waits for DV to be in a specific phase (Pending, Bound, Succeeded etc.)
// or garbage collected if the passed in phase is Succeeded
func WaitForDataVolumePhase(ci ClientsIface, namespace string, phase cdiv1.DataVolumePhase, dataVolumeName string) error {
	return WaitForDataVolumePhaseWithTimeout(ci, namespace, phase, dataVolumeName, dataVolumePhaseTime)
}

// WaitForDataVolumePhaseWithTimeout waits with timeout for DV to be in a specific phase (Pending, Bound, Succeeded etc.)
// or garbage collected if the passed in phase is Succeeded
func WaitForDataVolumePhaseWithTimeout(ci ClientsIface, namespace string, phase cdiv1.DataVolumePhase, dataVolumeName string, timeout time.Duration) error {
	var actualPhase cdiv1.DataVolumePhase
	if phase == cdiv1.Succeeded {
		cfg, err := ci.Cdi().CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if ttl := cc.GetDataVolumeTTLSeconds(cfg); ttl >= 0 {
			return WaitForDataVolumeGC(ci, namespace, dataVolumeName, ttl, dataVolumePhaseTime)
		}
	}
	err := wait.PollImmediate(dataVolumePollInterval, timeout, func() (bool, error) {
		dataVolume, err := ci.Cdi().CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
		if err != nil || dataVolume.Status.Phase != phase {
			actualPhase = dataVolume.Status.Phase
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("DataVolume %s not in phase %s within %v, actual phase=%s", dataVolumeName, phase, timeout, actualPhase)
	}
	return nil
}

// WaitForDataVolumeGC waits for DataVolume garbage collected
func WaitForDataVolumeGC(ci ClientsIface, namespace string, pvcName string, ttl int32, timeout time.Duration) error {
	var (
		actualPhase cdiv1.DataVolumePhase
		retain      bool
	)

	err := wait.PollImmediate(dataVolumePollInterval, timeout, func() (bool, error) {
		dv, err := ci.Cdi().CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{})
		if err == nil {
			retain = dv.Annotations[cc.AnnDeleteAfterCompletion] == "false"
			actualPhase = dv.Status.Phase
			if actualPhase != cdiv1.Succeeded {
				return false, nil
			}
			if retain {
				return true, nil
			}
			if ttl > 1 {
				// Check only once DV is retained for ttl/2 and GC'ed too early
				time.Sleep(time.Duration(ttl/2) * time.Second)
				if _, err = ci.Cdi().CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{}); err != nil {
					return false, fmt.Errorf("DataVolume %s was not retained for ttl, err %s", pvcName, err.Error())
				}
				ttl = 0
			}
			return false, nil
		}
		if !apierrs.IsNotFound(err) {
			return false, err
		}
		return true, nil
	})
	if retain {
		if err != nil {
			return err
		}
		if actualPhase != cdiv1.Succeeded {
			return fmt.Errorf("DataVolume %s is not in phase Succeeded within %v, actual phase=%s", pvcName, timeout, actualPhase)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("DataVolume %s was not garbage collected within %v err [%s] actual phase=%s", pvcName, timeout, err.Error(), actualPhase)
	}
	if _, err := ci.K8s().CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, metav1.GetOptions{}); err != nil {
		return err
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
			Source: &cdiv1.DataVolumeSource{
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
	// This function can work with DVs that use the 'Storage' field instead of the 'PVC' field
	spec := corev1.PersistentVolumeClaimSpec{}
	if datavolume.Spec.PVC != nil {
		spec = *datavolume.Spec.PVC
	}

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
		Spec: spec,
	}
}

// GetCloneType returnc the CDI clone type
func GetCloneType(clientSet *cdiclientset.Clientset, dataVolume *cdiv1.DataVolume) string {
	var cloneType string
	gomega.Eventually(func() bool {
		dv, err := clientSet.CdiV1beta1().DataVolumes(dataVolume.Namespace).Get(context.TODO(), dataVolume.Name, metav1.GetOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		val, ok := dv.Annotations["cdi.kubevirt.io/cloneType"]
		if ok {
			cloneType = val
		}
		return ok
	}, 90*time.Second, 2*time.Second).Should(gomega.BeTrue())
	return cloneType
}

// WaitForConditions waits until the data volume conditions match the expected conditions
func WaitForConditions(ci ClientsIface, dataVolumeName, namespace string, timeout, pollingInterval time.Duration, expectedConditions ...*cdiv1.DataVolumeCondition) {
	gomega.Eventually(func() bool {
		resultDv, dverr := ci.Cdi().CdiV1beta1().DataVolumes(namespace).Get(context.TODO(), dataVolumeName, metav1.GetOptions{})
		gomega.Expect(dverr).ToNot(gomega.HaveOccurred())
		return verifyConditions(resultDv.Status.Conditions, expectedConditions)
	}, timeout, pollingInterval).Should(gomega.BeTrue())
}

// verifyConditions checks if all the conditions exist and match testConditions
func verifyConditions(actualConditions []cdiv1.DataVolumeCondition, testConditions []*cdiv1.DataVolumeCondition) bool {
	for _, condition := range testConditions {
		if condition == nil {
			continue
		}
		actualCondition := dvc.FindConditionByType(condition.Type, actualConditions)
		if actualCondition == nil {
			return false
		}
		if actualCondition.Status != condition.Status {
			fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Condition.Status does not match for type: %s, status expected: [%s], status found: [%s]\n", condition.Type, condition.Status, actualCondition.Status)
			return false
		}
		if strings.Compare(actualCondition.Reason, condition.Reason) != 0 {
			fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Condition.Reason does not match for type: %s, reason expected [%s], reason found: [%s]\n", condition.Type, condition.Reason, actualCondition.Reason)
			return false
		}
		if !strings.Contains(actualCondition.Message, condition.Message) {
			fmt.Fprintf(ginkgo.GinkgoWriter, "INFO: Condition.Message does not match for type: %s, message expected: [%s],  message found: [%s]\n", condition.Type, condition.Message, actualCondition.Message)
			return false
		}
	}
	return true
}
