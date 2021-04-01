package utils

import (
	"context"

	k8sv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	cdiuploadv1 "kubevirt.io/containerized-data-importer/pkg/apis/upload/v1beta1"
	cdiClientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
)

const (
	// UploadFile is the file to upload
	UploadFile = "./images/tinyCore.iso"
	// UploadFileLargeVirtualDisk is the file to upload
	UploadFileLargeVirtualDisk = "./images/cirros-large-vdisk.qcow2"

	// UploadFileSize is the size of UploadFile
	UploadFileSize = 18874368

	// UploadFileMD5 is the expected MD5 of the uploaded file
	UploadFileMD5 = "2a7a52285c846314d1dbd79e9818270d"

	// UploadFileMD5100kbytes is the size of the image after being extended
	UploadFileMD5100kbytes = "3710416a680523c7d07538cb1026c60c"

	uploadTargetAnnotation = "cdi.kubevirt.io/storage.upload.target"
	uploadStatusAnnotation = "cdi.kubevirt.io/storage.pod.phase"
	uploadReadyAnnotation  = "cdi.kubevirt.io/storage.pod.ready"
)

// UploadPodName returns the name of the upload server pod associated with a PVC
func UploadPodName(pvc *k8sv1.PersistentVolumeClaim) string {
	return naming.GetResourceName("cdi-upload", pvc.Name)
}

// UploadPVCDefinition creates a PVC with the upload target annotation
func UploadPVCDefinition() *k8sv1.PersistentVolumeClaim {
	annotations := map[string]string{uploadTargetAnnotation: ""}
	return NewPVCDefinition("upload-test", "1Gi", annotations, nil)
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
