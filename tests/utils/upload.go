package utils

import (
	"context"

	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiuploadv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/upload/v1beta1"
	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
)

const (
	imagesPath = "./images"
	// TinyCoreFile is the file name of tine core
	TinyCoreFile = "/tinyCore.iso"
	// CirrosQCow2File is the file name of cirros qcow
	CirrosQCow2File = "/cirros-qcow2.img"
	// UploadFile is the file to upload
	UploadFile = imagesPath + TinyCoreFile
	// FsOverheadFile a file with some arbitrary size to check the fsOverhead validation logic
	FsOverheadFile = imagesPath + "/fs-overhead.qcow2"

	// UploadFileLargeVirtualDiskQcow is the file to upload (QCOW2)
	UploadFileLargeVirtualDiskQcow = "./images/cirros-large-virtual-size.qcow2"
	// UploadFileLargeVirtualDiskXz is the file to upload (XZ-compressed RAW file)
	UploadFileLargeVirtualDiskXz = "./images/cirros-large-virtual-size.raw.xz"
	// UploadFileLargePhysicalDiskQcow is the file to upload (QCOW2)
	UploadFileLargePhysicalDiskQcow = "./images/cirros-large-physical-size.qcow2"
	// UploadFileLargePhysicalDiskXz is the file to upload (XZ-compressed RAW file)
	UploadFileLargePhysicalDiskXz = "./images/cirros-large-physical-size.raw.xz"
	// UploadCirrosFile is the file to upload (QCOW2)
	UploadCirrosFile = imagesPath + CirrosQCow2File

	// UploadFileSize is the size of UploadFile
	UploadFileSize = 18874368
	// CirrosRawFileSize is the size of cirros.raw
	CirrosRawFileSize = 46137344

	// UploadFileMD5 is the expected MD5 of the uploaded file
	UploadFileMD5 = "2a7a52285c846314d1dbd79e9818270d"

	// UploadFileMD5100kbytes is the size of the image after being extended
	UploadFileMD5100kbytes = "3710416a680523c7d07538cb1026c60c"

	uploadTargetAnnotation      = "cdi.kubevirt.io/storage.upload.target"
	uploadStatusAnnotation      = "cdi.kubevirt.io/storage.pod.phase"
	uploadReadyAnnotation       = "cdi.kubevirt.io/storage.pod.ready"
	uploadContentTypeAnnotation = "cdi.kubevirt.io/storage.contentType"
)

// UploadPodName returns the name of the upload server pod associated with a PVC
func UploadPodName(pvc *k8sv1.PersistentVolumeClaim) string {
	uploadPodNameSuffix := pvc.Name
	if pvc.Spec.DataSourceRef != nil {
		uploadPodNameSuffix = populators.PVCPrimeName(pvc)
	}
	return naming.GetResourceName(common.UploadPodName, uploadPodNameSuffix)
}

// UploadPVCDefinition creates a PVC with the upload target annotation
func UploadPVCDefinition() *k8sv1.PersistentVolumeClaim {
	annotations := map[string]string{uploadTargetAnnotation: ""}
	return NewPVCDefinition("upload-test", "1Gi", annotations, nil)
}

// UploadArchivePVCDefinition creates a PVC with the upload target annotation and archive context-type
func UploadArchivePVCDefinition() *k8sv1.PersistentVolumeClaim {
	annotations := make(map[string]string)
	annotations[uploadTargetAnnotation] = ""
	annotations[uploadContentTypeAnnotation] = string(cdiv1.DataVolumeArchive)
	pvc := NewPVCDefinition("upload-archive-test", "1Gi", annotations, nil)
	return pvc
}

// UploadPopulationPVCDefinition creates a PVC with upload datasourceref
func UploadPopulationPVCDefinition() *k8sv1.PersistentVolumeClaim {
	pvcDef := NewPVCDefinition("upload-populator-pvc-test", "1Gi", nil, nil)
	apiGroup := cc.AnnAPIGroup
	pvcDef.Spec.DataSourceRef = &k8sv1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     cdiv1.VolumeUploadSourceRef,
		Name:     "upload-populator-test",
	}
	return pvcDef
}

// UploadPopulationBlockPVCDefinition creates a PVC with upload datasourceref
// and volumeMode 'Block'
func UploadPopulationBlockPVCDefinition(storageClassName string) *k8sv1.PersistentVolumeClaim {
	pvcDef := UploadPopulationPVCDefinition()
	pvcDef.Spec.StorageClassName = &storageClassName
	volumeMode := k8sv1.PersistentVolumeBlock
	pvcDef.Spec.VolumeMode = &volumeMode
	return pvcDef
}

// UploadPopulatorCR creates an upload source CR
func UploadPopulatorCR(namespace, contentType string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind":       cdiv1.VolumeUploadSourceRef,
			"apiVersion": "cdi.kubevirt.io/v1beta1",
			"metadata": map[string]interface{}{
				"name":      "upload-populator-test",
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"contentType": contentType,
			},
		},
	}
}

// UploadBlockPVCDefinition creates a PVC with the upload target annotation for block PV
func UploadBlockPVCDefinition(storageClass string) *k8sv1.PersistentVolumeClaim {
	annotations := map[string]string{uploadTargetAnnotation: ""}
	return NewBlockPVCDefinition("upload-test", "500Mi", annotations, nil, storageClass)
}

// WaitPVCUploadPodStatusRunning waits for the upload server pod status annotation to be Running
func WaitPVCUploadPodStatusRunning(clientSet *kubernetes.Clientset, pvc *k8sv1.PersistentVolumeClaim) (bool, error) {
	return WaitForPVCAnnotationWithValue(clientSet, pvc.Namespace, pvc, uploadStatusAnnotation, string(k8sv1.PodRunning))
}

// RequestUploadToken sends an upload token request to the server
func RequestUploadToken(clientSet *cdiClientset.Clientset, pvc *k8sv1.PersistentVolumeClaim) (string, error) {
	request := &cdiuploadv1.UploadTokenRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-token",
			Namespace: pvc.Namespace,
		},
		Spec: cdiuploadv1.UploadTokenRequestSpec{
			PvcName: pvc.Name,
		},
	}

	response, err := clientSet.UploadV1beta1().UploadTokenRequests(pvc.Namespace).Create(context.TODO(), request, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return response.Status.Token, nil
}
