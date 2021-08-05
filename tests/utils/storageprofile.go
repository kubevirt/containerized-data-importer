package utils

import (
	"context"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiclientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetStorageProfileSpec attempts to get the StorageProfile for storageClass by Name.
func GetStorageProfileSpec(clientSet *cdiclientset.Clientset, storageClassName string) (*cdiv1.StorageProfileSpec, error) {
	storageProfile, err := clientSet.CdiV1beta1().StorageProfiles().Get(context.TODO(), storageClassName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return &storageProfile.Spec, nil
}

// UpdateStorageProfile updates the storageProfile found by name, with given StorageProfileSpec.
func UpdateStorageProfile(client client.Client, name string, spec cdiv1.StorageProfileSpec) error {
	storageProfile := &cdiv1.StorageProfile{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name}, storageProfile)
	if err != nil {
		return err
	}
	storageProfile.Spec = spec
	err = client.Update(context.TODO(), storageProfile)
	if err != nil {
		return err
	}

	return nil
}

// ConfigureCloneStrategy updates the storageProfile found by name, with given CDICloneStrategy.
func ConfigureCloneStrategy(client client.Client,
	clientSet *cdiclientset.Clientset,
	storageClassName string,
	spec *cdiv1.StorageProfileSpec,
	cloneStrategy cdiv1.CDICloneStrategy) error {
	newProfileSpec := updateCloneStrategy(spec, cloneStrategy)
	if err := UpdateStorageProfile(client, storageClassName, *newProfileSpec); err != nil {
		return err
	}

	gomega.Eventually(func() *cdiv1.CDICloneStrategy {
		profile, err := clientSet.CdiV1beta1().StorageProfiles().Get(context.TODO(), storageClassName, metav1.GetOptions{})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
		if len(profile.Status.ClaimPropertySets) > 0 {
			return profile.Status.ClaimPropertySets[0].CloneStrategy
		}
		return nil
	}, time.Second*30, time.Second).Should(gomega.Equal(&cloneStrategy))

	return nil
}

func updateCloneStrategy(originalProfileSpec *cdiv1.StorageProfileSpec, cloneStrategy cdiv1.CDICloneStrategy) *cdiv1.StorageProfileSpec {
	newProfileSpec := originalProfileSpec.DeepCopy()

	if len(newProfileSpec.ClaimPropertySets) == 0 {
		newProfileSpec.ClaimPropertySets = []cdiv1.ClaimPropertySet{{CloneStrategy: &cloneStrategy}}
	} else {
		newProfileSpec.ClaimPropertySets[0].CloneStrategy = &cloneStrategy
	}

	return newProfileSpec
}
