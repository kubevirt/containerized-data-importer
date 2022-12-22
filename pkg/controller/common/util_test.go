package common

import (
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	table.DescribeTable("should return", func(sourceContentType, targetContentType string, expectedResult bool) {
		sourcePvc := CreatePvc("testPVC", "default", map[string]string{AnnContentType: sourceContentType}, nil)
		dvSpec := &cdiv1.DataVolumeSpec{}
		dvSpec.ContentType = cdiv1.DataVolumeContentType(targetContentType)

		validated, sourceContent, targetContent := validateContentTypes(sourcePvc, dvSpec)
		Expect(validated).To(Equal(expectedResult))
		Expect(sourceContent).To(Equal(getContentType(sourceContentType)))
		Expect(targetContent).To(Equal(getContentType(targetContentType)))
	},
		table.Entry("true when using archive in source and target", string(cdiv1.DataVolumeArchive), string(cdiv1.DataVolumeArchive), true),
		table.Entry("false when using archive in source and KubeVirt in target", string(cdiv1.DataVolumeArchive), string(cdiv1.DataVolumeKubeVirt), false),
		table.Entry("false when using KubeVirt in source and archive in target", string(cdiv1.DataVolumeKubeVirt), string(cdiv1.DataVolumeArchive), false),
		table.Entry("true when using KubeVirt in source and target", string(cdiv1.DataVolumeKubeVirt), string(cdiv1.DataVolumeKubeVirt), true),
		table.Entry("true when using default in source and target", "", "", true),
		table.Entry("true when using default in source and KubeVirt (explicit) in target", "", string(cdiv1.DataVolumeKubeVirt), true),
		table.Entry("true when using KubeVirt (explicit) in source and default in target", string(cdiv1.DataVolumeKubeVirt), "", true),
		table.Entry("false when using default in source and archive in target", "", string(cdiv1.DataVolumeArchive), false),
		table.Entry("false when using archive in source and default in target", string(cdiv1.DataVolumeArchive), "", false),
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
		sc, _ := GetDefaultStorageClass(client)
		Expect(sc.Name).To(Equal("test-storage-class-2"))
	})

	It("Should return nil if there's not default storage class", func() {
		client := CreateClient(
			CreateStorageClass("test-storage-class-1", nil),
			CreateStorageClass("test-storage-class-2", nil),
		)
		sc, _ := GetDefaultStorageClass(client)
		Expect(sc).To(BeNil())
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
