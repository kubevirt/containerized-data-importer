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
	n, err := CopyConfigMap(client, cdiNamespace, RegistryCertConfigMap, destNamespace, "", "")
	if err != nil {
		return "", err
	}
	return n, nil
}

// CopyRegistryCertConfigMapDestName copies the test registry configmap, it assumes the Registry host is in the CDI namespace
func CopyRegistryCertConfigMapDestName(client kubernetes.Interface, destNamespace, cdiNamespace, destName string) (string, error) {
	n, err := CopyConfigMap(client, cdiNamespace, RegistryCertConfigMap, destNamespace, destName, "")
	if err != nil {
		return "", err
	}
	return n, nil
}

// CopyFileHostCertConfigMap copies the test file host configmap, it assumes the File host is in the CDI namespace
func CopyFileHostCertConfigMap(client kubernetes.Interface, destNamespace, cdiNamespace string) (string, error) {
	n, err := CopyConfigMap(client, cdiNamespace, FileHostCertConfigMap, destNamespace, "", "")
	if err != nil {
		return "", err
	}
	return n, nil
}

// CopyImageIOCertConfigMap copies the test imageio configmap, it assumes the imageio server is in the CDI namespace
func CopyImageIOCertConfigMap(client kubernetes.Interface, destNamespace, cdiNamespace string) (string, error) {
	n, err := CopyConfigMap(client, cdiNamespace, ImageIOCertConfigMap, destNamespace, "", "")
	if err != nil {
		return "", err
	}
	return n, nil
}

// CopyConfigMap copies a ConfigMap, set destKey if you want to override the default tls.crt with a different key name
func CopyConfigMap(client kubernetes.Interface, srcNamespace, srcName, destNamespace, destName, destKey string) (string, error) {
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

	// Use this when overriding the default key when copying the configmap.
	if destKey != "" {
		data := make(map[string]string, 0)
		for _, v := range src.Data {
			data[destKey] = v
		}
		dst.Data = data
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

// CreateCertConfigMapWeirdFilename copies a configmap with a different key value
func CreateCertConfigMapWeirdFilename(client kubernetes.Interface, destNamespace, srcNamespace string) (string, error) {
	var certBytes string
	srcName := FileHostCertConfigMap
	srcCm, err := client.CoreV1().ConfigMaps(srcNamespace).Get(context.TODO(), srcName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	for _, value := range srcCm.Data {
		certBytes = value
		break
	}
	destName := srcName + "-" + strings.ToLower(util.RandAlphaNum(8))
	dst := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: destName,
		},
		Data: map[string]string{
			"weird-filename-should-still-be-accepted.crt": certBytes,
		},
	}
	_, err = client.CoreV1().ConfigMaps(destNamespace).Create(context.TODO(), dst, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return destName, nil
}

// DeleteConfigMap deletes a ConfigMap
func DeleteConfigMap(client kubernetes.Interface, namespace, name string) error {
	err := client.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}
