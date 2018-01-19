package clients

import (
	"github.com/minio/minio-go"
	"github.com/golang/glog"
	"fmt"
	k8sRest "k8s.io/client-go/rest"
	clientset "k8s.io/client-go/kubernetes"
)

// Returns a minio api client.
func GetMinioS3Client(acct, user, pass, ip string) (*minio.Client, error) {
	glog.Infof("Creating s3 client based on: \"%s:%s\" on ip %s", acct, user, ip)

	id := fmt.Sprintf("%s:%s", acct, user)
	useSSL := false
	minioClient, err := minio.NewV2(ip, id, pass, useSSL)
	if err != nil {
		return nil, fmt.Errorf("Unable to create minio S3 client: %v", err)
	}
	return minioClient, nil
}

// Returns a k8s api client.
func GetKubeClient() (*clientset.Clientset, error) {
	glog.Info("Getting k8s API Client config")
	kubeClientConfig, err := k8sRest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to create k8s in-cluster config: %v", err)
	}
	glog.Info("Creating new Kubernetes Clientset")
	cs, err := clientset.NewForConfig(kubeClientConfig)
	return cs, err
}