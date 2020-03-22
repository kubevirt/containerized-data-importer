package tests_test

import (
	"fmt"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	secclient "github.com/openshift/client-go/security/clientset/versioned"
	conditions "github.com/openshift/custom-resource-status/conditions/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	cdiv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	operatorcontroller "kubevirt.io/containerized-data-importer/pkg/operator/controller"
	"kubevirt.io/containerized-data-importer/tests/framework"
	"kubevirt.io/containerized-data-importer/tests/utils"
)

var _ = Describe("Operator tests", func() {
	f := framework.NewFrameworkOrDie("operator-test")

	It("should create a route in OpenShift", func() {
		if !controller.IsOpenshift(f.K8sClient) {
			Skip("This test is OpenShift specific")
		}

		routeClient, err := routeclient.NewForConfig(f.RestConfig)
		Expect(err).ToNot(HaveOccurred())

		r, err := routeClient.RouteV1().Routes(f.CdiInstallNs).Get("cdi-uploadproxy", metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		Expect(r.Spec.TLS.Termination).To(Equal(routev1.TLSTerminationReencrypt))
	})

	It("add cdi-sa to anyuid scc", func() {
		if !controller.IsOpenshift(f.K8sClient) {
			Skip("This test is OpenShift specific")
		}

		secClient, err := secclient.NewForConfig(f.RestConfig)
		Expect(err).ToNot(HaveOccurred())

		scc, err := secClient.SecurityV1().SecurityContextConstraints().Get("anyuid", metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		cdiSA := fmt.Sprintf("system:serviceaccount:%s:cdi-sa", f.CdiInstallNs)
		Expect(scc.Users).Should(ContainElement(cdiSA))
	})

	// Condition flags can be found here with their meaning https://github.com/kubevirt/hyperconverged-cluster-operator/blob/master/docs/conditions.md
	It("Condition flags on CR should be healthy and operating", func() {
		cdiObjects, err := f.CdiClient.CdiV1alpha1().CDIs().List(metav1.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(cdiObjects.Items)).To(Equal(1))
		cdiObject := cdiObjects.Items[0]
		conditionMap := operatorcontroller.GetConditionValues(cdiObject.Status.Conditions)
		// Application should be fully operational and healthy.
		Expect(conditionMap[conditions.ConditionAvailable]).To(Equal(corev1.ConditionTrue))
		Expect(conditionMap[conditions.ConditionProgressing]).To(Equal(corev1.ConditionFalse))
		Expect(conditionMap[conditions.ConditionDegraded]).To(Equal(corev1.ConditionFalse))
	})

	It("should deploy components that tolerate CriticalAddonsOnly taint", func() {
		var cdiPods *corev1.PodList
		var err error
		By("finding all nodes that are running cdi components")
		cdiPods, err = f.K8sClient.CoreV1().Pods(f.CdiInstallNs).List(metav1.ListOptions{})
		Expect(err).ShouldNot(HaveOccurred(), "failed listing cdi pods")
		Expect(len(cdiPods.Items)).To(BeNumerically(">", 0), "no cdi pods found")

		By("setting a watch for terminated cdi pods")
		lw, err := f.K8sClient.CoreV1().Pods(f.CdiInstallNs).Watch(metav1.ListOptions{})
		Expect(err).ShouldNot(HaveOccurred())
		signalTerminatedPods := func(stopCn <-chan bool, eventsCn <-chan watch.Event, terminatedPodsCn chan<- bool) {
			for {
				select {
				case <-stopCn:
					return
				case e := <-eventsCn:
					pod, ok := e.Object.(*corev1.Pod)
					Expect(ok).To(BeTrue())
					if _, isTestingComponent := pod.Labels["cdi.kubevirt.io/testing"]; isTestingComponent {
						continue
					}
					if pod.DeletionTimestamp != nil {
						fmt.Fprintf(GinkgoWriter, "DEBUG: Pod marked for deletion: %+v\n", pod)
						fmt.Fprintf(GinkgoWriter, "DEBUG: Deletion timestamp: %s\n", pod.DeletionTimestamp.String())
						terminatedPodsCn <- true
						return
					}
				}
			}
		}
		stopCn := make(chan bool, 1)
		terminatedPodsCn := make(chan bool)
		go signalTerminatedPods(stopCn, lw.ResultChan(), terminatedPodsCn)

		// nodes that run cdi components.
		cdiNodes := make(map[string]*corev1.Node)
		By("adding taints to any node that runs cdi component")
		criticalPodTaint := corev1.Taint{
			Key:    "CriticalAddonsOnly",
			Value:  "",
			Effect: corev1.TaintEffectNoExecute,
		}

		for _, cdiPod := range cdiPods.Items {
			cdiNode, err := f.K8sClient.CoreV1().Nodes().Get(cdiPod.Spec.NodeName, metav1.GetOptions{})
			Expect(err).ShouldNot(HaveOccurred(), "failed retrieving node")
			hasTaint := false
			// check if node already has the taint
			for _, taint := range cdiNode.Spec.Taints {
				if reflect.DeepEqual(taint, criticalPodTaint) {
					// node already have the taint set
					hasTaint = true
					break
				}
			}
			if hasTaint {
				continue
			}
			cdiNode.ResourceVersion = ""
			cdiNodes[cdiPod.Spec.NodeName] = cdiNode
			cdiNodeCopy := cdiNode.DeepCopy()
			cdiNodeCopy.Spec.Taints = append(cdiNodeCopy.Spec.Taints, criticalPodTaint)
			_, err = f.K8sClient.CoreV1().Nodes().Update(cdiNodeCopy)
			Expect(err).ShouldNot(HaveOccurred(), "failed setting taint on node")
		}
		defer func() {
			var errors []error
			By("restoring nodes")
			for _, nodeSpec := range cdiNodes {
				_, err := f.K8sClient.CoreV1().Nodes().Update(nodeSpec)
				if err != nil {
					errors = append(errors, err)
				}
			}
			Expect(errors).Should(BeEmpty(), "failed restoring one or more nodes")
		}()

		Consistently(terminatedPodsCn, 5*time.Second).
			ShouldNot(Receive(), "pods should not terminate")
		stopCn <- true
	})
})

var _ = Describe("Operator delete CDI tests", func() {
	var cr *cdiv1alpha1.CDI
	f := framework.NewFrameworkOrDie("operator-delete-cdi-test")

	BeforeEach(func() {
		var err error
		cr, err = f.CdiClient.CdiV1alpha1().CDIs().Get("cdi", metav1.GetOptions{})
		if errors.IsNotFound(err) {
			Skip("CDI CR 'cdi' does not exist.  Probably managed by another operator so skipping.")
		}
		Expect(err).ToNot(HaveOccurred())
	})

	ensureCDI := func() {
		if cr == nil {
			return
		}

		cdi, err := f.CdiClient.CdiV1alpha1().CDIs().Get(cr.Name, metav1.GetOptions{})
		if err == nil {
			if cdi.DeletionTimestamp == nil {
				cdi.Spec = cr.Spec
				_, err = f.CdiClient.CdiV1alpha1().CDIs().Update(cdi)
				Expect(err).ToNot(HaveOccurred())
				return
			}

			Eventually(func() bool {
				_, err = f.CdiClient.CdiV1alpha1().CDIs().Get(cr.Name, metav1.GetOptions{})
				if errors.IsNotFound(err) {
					return true
				}
				Expect(err).ToNot(HaveOccurred())
				return false
			}, 5*time.Minute, 2*time.Second).Should(BeTrue())
		} else {
			Expect(errors.IsNotFound(err)).To(BeTrue())
		}

		cdi = &cdiv1alpha1.CDI{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cdi",
			},
			Spec: cr.Spec,
		}

		cdi, err = f.CdiClient.CdiV1alpha1().CDIs().Create(cdi)
		Expect(err).ToNot(HaveOccurred())

		Eventually(func() bool {
			cdi, err = f.CdiClient.CdiV1alpha1().CDIs().Get(cr.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			Expect(cdi.Status.Phase).ShouldNot(Equal(cdiv1alpha1.CDIPhaseError))
			for _, c := range cdi.Status.Conditions {
				if c.Type == conditions.ConditionAvailable && c.Status == corev1.ConditionTrue {
					return true
				}
			}
			return false
		}, 10*time.Minute, 2*time.Second).Should(BeTrue())
	}

	AfterEach(func() {
		ensureCDI()
	})

	It("should remove/install CDI a number of times successfully", func() {
		for i := 0; i < 10; i++ {
			err := f.CdiClient.CdiV1alpha1().CDIs().Delete(cr.Name, &metav1.DeleteOptions{})
			Expect(err).ToNot(HaveOccurred())
			ensureCDI()
		}
	})

	It("should delete an upload pod", func() {
		dv := utils.NewDataVolumeForUpload("delete-me", "1Gi")

		By("Creating datavolume")
		dv, err := utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		By("Waiting for pod to be running")
		Eventually(func() bool {
			pod, err := f.K8sClient.CoreV1().Pods(dv.Namespace).Get("cdi-upload-"+dv.Name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return false
			}
			Expect(err).ToNot(HaveOccurred())
			return pod.Status.Phase == corev1.PodRunning
		}, 2*time.Minute, 1*time.Second).Should(BeTrue())

		By("Deleting CDI")
		err = f.CdiClient.CdiV1alpha1().CDIs().Delete(cr.Name, &metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Waiting for pod to be deleted")
		Eventually(func() bool {
			_, err = f.K8sClient.CoreV1().Pods(dv.Namespace).Get("cdi-upload-"+dv.Name, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return true
			}
			Expect(err).ToNot(HaveOccurred())
			return false
		}, 2*time.Minute, 1*time.Second).Should(BeTrue())
	})

	It("should block CDI delete", func() {
		uninstallStrategy := cdiv1alpha1.CDIUninstallStrategyBlockUninstallIfWorkloadsExist

		By("Getting CDI resource")
		cdi, err := f.CdiClient.CdiV1alpha1().CDIs().Get(cr.Name, metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())

		cdi.Spec.UninstallStrategy = &uninstallStrategy
		_, err = f.CdiClient.CdiV1alpha1().CDIs().Update(cdi)
		Expect(err).ToNot(HaveOccurred())

		By("Waiting for update")
		Eventually(func() bool {
			cdi, err = f.CdiClient.CdiV1alpha1().CDIs().Get(cr.Name, metav1.GetOptions{})
			Expect(err).ToNot(HaveOccurred())
			return cdi.Spec.UninstallStrategy != nil && *cdi.Spec.UninstallStrategy == uninstallStrategy
		}, 2*time.Minute, 1*time.Second).Should(BeTrue())

		By("Creating datavolume")
		dv := utils.NewDataVolumeForUpload("delete-me", "1Gi")
		dv, err = utils.CreateDataVolumeFromDefinition(f.CdiClient, f.Namespace.Name, dv)
		Expect(err).ToNot(HaveOccurred())

		By("Cannot delete CDI")
		err = f.CdiClient.CdiV1alpha1().CDIs().Delete(cr.Name, &metav1.DeleteOptions{DryRun: []string{"All"}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("there are still DataVolumes present"))

		err = f.CdiClient.CdiV1alpha1().DataVolumes(f.Namespace.Name).Delete(dv.Name, &metav1.DeleteOptions{})
		Expect(err).ToNot(HaveOccurred())

		By("Can delete CDI")
		err = f.CdiClient.CdiV1alpha1().CDIs().Delete(cr.Name, &metav1.DeleteOptions{DryRun: []string{"All"}})
		Expect(err).ToNot(HaveOccurred())
	})
})
