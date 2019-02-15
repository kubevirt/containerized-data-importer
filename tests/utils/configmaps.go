package utils

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CopyRegistryCertConfigMap copies the test registry ConfigMap
func CopyRegistryCertConfigMap(client kubernetes.Interface, destNamespace string) (string, error) {
	err := CopyConfigMap(client, RegistryHostNs, RegistryConfigMap, destNamespace, RegistryConfigMap)
	if err != nil {
		return "", err
	}
	return RegistryConfigMap, nil
}

// CopyConfigMap copies a ConfigMap
func CopyConfigMap(client kubernetes.Interface, srcNamespace, srcName, destNamespace, destName string) error {
	src, err := client.CoreV1().ConfigMaps(srcNamespace).Get(srcName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	dst := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: srcName,
		},
		Data: src.Data,
	}

	err = client.CoreV1().ConfigMaps(destNamespace).Delete(destName, nil)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	_, err = client.CoreV1().ConfigMaps(destNamespace).Create(dst)
	if err != nil {
		return nil
	}

	return nil
}
