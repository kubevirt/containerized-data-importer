package tests_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/tests/framework"
)

/*
 * Tests that use artifacts created on older version.
 * Artifacts are created by manifests/templates/upgrade-testing-artifacts.yaml.in
 */

const oldVersionArtifactsNamespace = "cdi-testing-old-version-artifacts"

var _ = Describe("[Upgrade]", Serial, func() {
	f := framework.NewFramework("upgrade-test")

	BeforeEach(func() {
		_, err := f.K8sClient.CoreV1().Namespaces().Get(context.TODO(), oldVersionArtifactsNamespace, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			Skip(fmt.Sprintf("Not setup to perform upgrade testing; missing namespace %s", oldVersionArtifactsNamespace))
		}
	})

	DescribeTable("[rfe_id:5493]DV status.name is populated after upgrade", func(dvName string) {
		dv, err := f.CdiClient.CdiV1beta1().DataVolumes(oldVersionArtifactsNamespace).Get(context.TODO(), dvName, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			_, err := f.K8sClient.CoreV1().PersistentVolumeClaims(oldVersionArtifactsNamespace).Get(context.TODO(), dvName, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return
		}
		Expect(err).ToNot(HaveOccurred())
		Expect(dv.Status.ClaimName).To(Equal(dvName))
	},
		Entry("[test_id:7715]with v1beta1 datavolume", "olddv"),
	)
})
