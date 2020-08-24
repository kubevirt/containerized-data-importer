package utils

import (
	"context"

	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetOrCreateNamespace returns a namespace if it exists, and creates it if it does not
func GetOrCreateNamespace(c *kubernetes.Clientset, name string) (*v1.Namespace, error) {
	ns, err := c.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	if !k8serrors.IsNotFound(err) {
		return ns, err
	}

	spec := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	ns, err = c.CoreV1().Namespaces().Create(context.TODO(), spec, metav1.CreateOptions{})
	return ns, err
}
