package utils

import (
	"context"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"kubevirt.io/containerized-data-importer/pkg/util"
)

// CopyRegistryCertConfigMap copies the test registry configmap, it assumes the Registry host is in the CDI namespace
func CopyRegistryCertConfigMap(client kubernetes.Interface, destNamespace, cdiNamespace string) (string, error) {
	n, err := CopyConfigMap(client, cdiNamespace, RegistryCertConfigMap, destNamespace, "")
	if err != nil {
		return "", err
	}
	return n, nil
}

// CopyRegistryCertConfigMapDestName copies the test registry configmap, it assumes the Registry host is in the CDI namespace
func CopyRegistryCertConfigMapDestName(client kubernetes.Interface, destNamespace, cdiNamespace, destName string) (string, error) {
	n, err := CopyConfigMap(client, cdiNamespace, RegistryCertConfigMap, destNamespace, destName)
	if err != nil {
		return "", err
	}
	return n, nil
}

// CopyFileHostCertConfigMap copies the test file host configmap, it assumes the File host is in the CDI namespace
func CopyFileHostCertConfigMap(client kubernetes.Interface, destNamespace, cdiNamespace string) (string, error) {
	n, err := CopyConfigMap(client, cdiNamespace, FileHostCertConfigMap, destNamespace, "")
	if err != nil {
		return "", err
	}
	return n, nil
}

// CopyImageIOCertConfigMap copies the test imageio configmap, it assumes the imageio server is in the CDI namespace
func CopyImageIOCertConfigMap(client kubernetes.Interface, destNamespace, cdiNamespace string) (string, error) {
	n, err := CopyConfigMap(client, cdiNamespace, ImageIOCertConfigMap, destNamespace, "")
	if err != nil {
		return "", err
	}
	return n, nil
}

// CopyConfigMap copies a ConfigMap
func CopyConfigMap(client kubernetes.Interface, srcNamespace, srcName, destNamespace, destName string) (string, error) {
	src, err := client.CoreV1().ConfigMaps(srcNamespace).Get(context.TODO(), srcName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	if destName == "" {
		destName = srcName + "-" + strings.ToLower(util.RandAlphaNum(8))
	}

	dst := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: destName,
		},
		Data: src.Data,
	}

	err = client.CoreV1().ConfigMaps(destNamespace).Delete(context.TODO(), destName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return "", err
	}

	_, err = client.CoreV1().ConfigMaps(destNamespace).Create(context.TODO(), dst, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return destName, nil
}
