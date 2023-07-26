package common

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

var _ = Describe("GetRequestedImageSize", func() {
	It("Should return 1G if 1G provided", func() {
		result, err := GetRequestedImageSize(CreatePvc("testPVC", "default", nil, nil))
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("1G"))
	})

	It("Should return error and blank if no size provided", func() {
		result, err := GetRequestedImageSize(createPvcNoSize("testPVC", "default", nil, nil))
		Expect(err).To(HaveOccurred())
		Expect(result).To(Equal(""))
	})
})

var _ = Describe("validateContentTypes", func() {
	getContentType := func(contentType string) cdiv1.DataVolumeContentType {
		if contentType == "" {
			return cdiv1.DataVolumeKubeVirt
		}
		return cdiv1.DataVolumeContentType(contentType)
	}

	DescribeTable("should return", func(sourceContentType, targetContentType string, expectedResult bool) {
		sourcePvc := CreatePvc("testPVC", "default", map[string]string{AnnContentType: sourceContentType}, nil)
		dvSpec := &cdiv1.DataVolumeSpec{}
		dvSpec.ContentType = cdiv1.DataVolumeContentType(targetContentType)

		validated, sourceContent, targetContent := validateContentTypes(sourcePvc, dvSpec)
		Expect(validated).To(Equal(expectedResult))
		Expect(sourceContent).To(Equal(getContentType(sourceContentType)))
		Expect(targetContent).To(Equal(getContentType(targetContentType)))
	},
		Entry("true when using archive in source and target", string(cdiv1.DataVolumeArchive), string(cdiv1.DataVolumeArchive), true),
		Entry("false when using archive in source and KubeVirt in target", string(cdiv1.DataVolumeArchive), string(cdiv1.DataVolumeKubeVirt), false),
		Entry("false when using KubeVirt in source and archive in target", string(cdiv1.DataVolumeKubeVirt), string(cdiv1.DataVolumeArchive), false),
		Entry("true when using KubeVirt in source and target", string(cdiv1.DataVolumeKubeVirt), string(cdiv1.DataVolumeKubeVirt), true),
		Entry("true when using default in source and target", "", "", true),
		Entry("true when using default in source and KubeVirt (explicit) in target", "", string(cdiv1.DataVolumeKubeVirt), true),
		Entry("true when using KubeVirt (explicit) in source and default in target", string(cdiv1.DataVolumeKubeVirt), "", true),
		Entry("false when using default in source and archive in target", "", string(cdiv1.DataVolumeArchive), false),
		Entry("false when using archive in source and default in target", string(cdiv1.DataVolumeArchive), "", false),
	)
})

var _ = Describe("GetDefaultStorageClass", func() {
	It("Should return the default storage class name", func() {
		client := CreateClient(
			CreateStorageClass("test-storage-class-1", nil),
			CreateStorageClass("test-storage-class-2", map[string]string{
				AnnDefaultStorageClass: "true",
			}),
		)
		sc, _ := GetDefaultStorageClass(context.Background(), client)
		Expect(sc.Name).To(Equal("test-storage-class-2"))
	})

	It("Should return nil if there's not default storage class", func() {
		client := CreateClient(
			CreateStorageClass("test-storage-class-1", nil),
			CreateStorageClass("test-storage-class-2", nil),
		)
		sc, _ := GetDefaultStorageClass(context.Background(), client)
		Expect(sc).To(BeNil())
	})
})

var _ = Describe("Rebind", func() {
	It("Should return error if PV doesn't exist", func() {
		client := CreateClient()
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPVC",
				Namespace: "namespace",
			},
			Spec: v1.PersistentVolumeClaimSpec{
				VolumeName: "testPV",
			},
		}
		err := Rebind(context.Background(), client, pvc, pvc)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("Should return error if bound to unexpected claim", func() {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPVC",
				Namespace: "namespace",
			},
			Spec: v1.PersistentVolumeClaimSpec{
				VolumeName: "testPV",
			},
		}
		pv := &v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testPV",
			},
			Spec: v1.PersistentVolumeSpec{
				ClaimRef: &v1.ObjectReference{
					Name:      "anotherPVC",
					Namespace: "namespace",
					UID:       "uid",
				},
			},
		}
		client := CreateClient(pv)
		err := Rebind(context.Background(), client, pvc, pvc)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("PV testPV bound to unexpected claim anotherPVC"))
	})
	It("Should return nil if bound to target claim", func() {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPVC",
				Namespace: "namespace",
			},
			Spec: v1.PersistentVolumeClaimSpec{
				VolumeName: "testPV",
			},
		}
		targetPVC := pvc.DeepCopy()
		targetPVC.Name = "targetPVC"
		targetPVC.UID = "uid"
		pv := &v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testPV",
			},
			Spec: v1.PersistentVolumeSpec{
				ClaimRef: &v1.ObjectReference{
					Name:      "targetPVC",
					Namespace: "namespace",
					UID:       "uid",
				},
			},
		}
		client := CreateClient(pv)
		err := Rebind(context.Background(), client, pvc, targetPVC)
		Expect(err).ToNot(HaveOccurred())
	})
	It("Should rebind pv to target claim", func() {
		pvc := &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testPVC",
				Namespace: "namespace",
			},
			Spec: v1.PersistentVolumeClaimSpec{
				VolumeName: "testPV",
			},
		}
		targetPVC := pvc.DeepCopy()
		targetPVC.Name = "targetPVC"
		pvc.UID = "uid"
		pv := &v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "testPV",
			},
			Spec: v1.PersistentVolumeSpec{
				ClaimRef: &v1.ObjectReference{
					Name:      "testPVC",
					Namespace: "namespace",
					UID:       "uid",
				},
			},
		}
		AddAnnotation(pv, "someAnno", "somevalue")
		client := CreateClient(pv)
		err := Rebind(context.Background(), client, pvc, targetPVC)
		Expect(err).ToNot(HaveOccurred())
		updatedPV := &v1.PersistentVolume{}
		key := types.NamespacedName{Name: pv.Name, Namespace: pv.Namespace}
		err = client.Get(context.TODO(), key, updatedPV)
		Expect(err).ToNot(HaveOccurred())
		Expect(updatedPV.Spec.ClaimRef.Name).To(Equal(targetPVC.Name))
		//make sure annotations of pv from before rebind dont get deleted
		Expect(pv.Annotations["someAnno"]).To(Equal("somevalue"))
	})
})

func createPvcNoSize(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
			Labels:      labels,
		},
	}
}
