package tests_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"

	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("Upload tests", func() {

	f := framework.NewFrameworkOrDie("upload-func-test")

	It("Upload to PVC should succeed", func() {

		By("Creating PVC with upload target annotation")
		pvc, err := f.CreatePVCFromDefinition(utils.UploadPVCDefinition())
		Expect(err).ToNot(HaveOccurred())

		By("Verify that upload server POD running")
		err = f.WaitTimeoutForPodReady(utils.UploadPodName(pvc), time.Second*20)
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

		By("Do upload")
		err = utils.UploadImageFromNode(f.K8sClient, f.GoCLIPath, token)
		Expect(err).ToNot(HaveOccurred())

		err = f.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, pvc.Name)
		Expect(err).ToNot(HaveOccurred())

		By("Verify content")
		same := f.VerifyTargetPVCContentMD5(f.Namespace, pvc, utils.DefaultImagePath, utils.UploadFileMD5)
		Expect(same).To(BeTrue())

		By("Delete upload PVC")
		err = f.DeletePVC(pvc)
		Expect(err).ToNot(HaveOccurred())

		By("Wait for upload pod to be deleted")
		deleted, err := utils.WaitPodDeleted(f.K8sClient, utils.UploadPodName(pvc), f.Namespace.Name, time.Second*20)
		Expect(err).ToNot(HaveOccurred())
		Expect(deleted).To(BeTrue())
	})
})
