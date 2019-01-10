package tests_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"

	"github.com/onsi/ginkgo/extensions/table"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("[rfe_id:138][crit:high][vendor:cnv-qe@redhat.com][level:component]Upload tests", func() {

	var pvc *v1.PersistentVolumeClaim
	var err error

	f := framework.NewFrameworkOrDie("upload-func-test")

	BeforeEach(func() {
		if pvc != nil {
			Eventually(func() bool {
				// Make sure the pvc doesn't still exist. The after each should have called delete.
				_, err := f.FindPVC(pvc.Name)
				return err != nil
			}, timeout, pollingInterval).Should(BeTrue())
		}
		By("Creating PVC with upload target annotation")
		pvc, err = f.CreatePVCFromDefinition(utils.UploadPVCDefinition())
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		By("Delete upload PVC")
		err = f.DeletePVC(pvc)
		Expect(err).ToNot(HaveOccurred())

		By("Wait for upload pod to be deleted")
		deleted, err := utils.WaitPodDeleted(f.K8sClient, utils.UploadPodName(pvc), f.Namespace.Name, time.Second*20)
		Expect(err).ToNot(HaveOccurred())
		Expect(deleted).To(BeTrue())
	})

	table.DescribeTable("should", func(validToken bool) {

		By("Verify that upload server POD running")
		err := f.WaitTimeoutForPodReady(utils.UploadPodName(pvc), time.Second*20)
		Expect(err).ToNot(HaveOccurred())

		By("Verify PVC status annotation says running")
		found, err := utils.WaitPVCUploadPodStatusRunning(f.K8sClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(found).To(BeTrue())

		By("Prep for upload")
		err = utils.DownloadImageToNode(f.K8sClient, f.GoCLIPath)
		Expect(err).ToNot(HaveOccurred())

		By("Get an upload token")
		token, err := utils.RequestUploadToken(f.CdiClient, pvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeEmpty())

		if !validToken {
			token = "abc"
		}

		By("Do upload")
		err = utils.UploadImageFromNode(f.K8sClient, f.GoCLIPath, token)
		Expect(err).ToNot(HaveOccurred())

		err = f.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		if !validToken {
			By("Get an error while verifying content")
			_, err = f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.UploadFileMD5)
			Expect(err).To(HaveOccurred())
		} else {
			By("Verify content")
			same, err := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.UploadFileMD5)
			Expect(err).ToNot(HaveOccurred())
			Expect(same).To(BeTrue())
		}
	},
		table.Entry("[test_id:1368]succeed given a valid token", true),
		table.Entry("[posneg:negative][test_id:1369]fail given an invalid token", false),
	)
})
