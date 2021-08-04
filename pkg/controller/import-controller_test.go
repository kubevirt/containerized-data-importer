/*
Copyright 2020 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controller

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"

	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
)

const (
	testEndPoint   = "http://test.somewhere.tt.blah"
	testImage      = "test/image"
	testPullPolicy = "Always"
)

var (
	testStorageClass = "test-sc"
	importLog        = logf.Log.WithName("import-controller-test")
)

var _ = Describe("Test PVC annotations status", func() {

	It("Should return complete if annotation is set", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{AnnPodPhase: string(corev1.PodSucceeded)}, nil)
		Expect(isPVCComplete(testPvc)).To(BeTrue())
	})

	It("Should NOT return complete if annotation is not succeeded", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{AnnPodPhase: string(corev1.PodPending)}, nil)
		Expect(isPVCComplete(testPvc)).To(BeFalse())
	})

	It("Should NOT return complete if annotation is missing", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{}, nil)
		Expect(isPVCComplete(testPvc)).To(BeFalse())
	})

	It("Should be interesting if NOT complete, and endpoint and source is set", func() {
		r := createImportReconciler()
		testPvc := createPvc("testPvc1", "default", map[string]string{AnnPodPhase: string(corev1.PodPending), AnnEndpoint: testEndPoint, AnnSource: SourceHTTP}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeTrue())
	})

	It("Should NOT be interesting if complete, and endpoint and source is set", func() {
		r := createImportReconciler()
		testPvc := createPvc("testPvc1", "default", map[string]string{AnnPodPhase: string(corev1.PodSucceeded), AnnEndpoint: testEndPoint, AnnSource: SourceHTTP}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeFalse())
	})

	It("Should be interesting if NOT complete, and endpoint missing and source is set", func() {
		r := createImportReconciler()
		testPvc := createPvc("testPvc1", "default", map[string]string{AnnPodPhase: string(corev1.PodRunning), AnnSource: SourceHTTP}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeTrue())
	})

	It("Should be interesting if NOT complete, and endpoint set and source is missing", func() {
		r := createImportReconciler()
		testPvc := createPvc("testPvc1", "default", map[string]string{AnnPodPhase: string(corev1.PodPending), AnnEndpoint: testEndPoint}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeTrue())
	})

	It("Should NOT be interesting if NOT BOUND, and endpoint and source is set, and honorWaitForFirstConsumerEnabled", func() {
		r := createImportReconciler()
		r.featureGates = &FakeFeatureGates{honorWaitForFirstConsumerEnabled: true}
		testPvc := createPendingPvc("testPvc1", "default", map[string]string{AnnPodPhase: string(corev1.PodPending), AnnEndpoint: testEndPoint, AnnSource: SourceHTTP}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeFalse())
	})

	It("Should be interesting if NOT BOUND, and endpoint and source is set, and honorWaitForFirstConsumerEnabled and isImmediateBindingRequested is requested", func() {
		r := createImportReconciler()
		r.featureGates = &FakeFeatureGates{honorWaitForFirstConsumerEnabled: true}
		testPvc := createPendingPvc("testPvc1", "default", map[string]string{
			AnnPodPhase:         string(corev1.PodPending),
			AnnEndpoint:         testEndPoint,
			AnnSource:           SourceHTTP,
			AnnImmediateBinding: "true",
		}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeTrue())
	})
	It("Should be interesting if NOT BOUND, and endpoint and source is set, and honorWaitForFirstConsumerEnabled is false and isImmediateBindingRequested is requested", func() {
		r := createImportReconciler()
		r.featureGates = &FakeFeatureGates{honorWaitForFirstConsumerEnabled: false}
		testPvc := createPendingPvc("testPvc1", "default", map[string]string{
			AnnPodPhase:         string(corev1.PodPending),
			AnnEndpoint:         testEndPoint,
			AnnSource:           SourceHTTP,
			AnnImmediateBinding: "true",
		}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeTrue())
	})
})

var _ = Describe("ImportConfig Controller reconcile loop", func() {
	var (
		reconciler *ImportReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			reconciler = nil
		}
	})

	It("Should return success if a PVC with no annotations is passed, due to it being ignored", func() {
		reconciler = createImportReconciler(createPvc("testPvc1", "default", map[string]string{}, nil))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should return success if no PVC can be found, due to it not existing", func() {
		reconciler = createImportReconciler()
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should return success if no PVC can be found due to not existing in passed namespace", func() {
		reconciler = createImportReconciler(createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint}, nil))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "invalid"}})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should succeed and be marked complete, if creating a block PVC with source none", func() {
		reconciler = createImportReconciler(createBlockPvc("testPvc1", "block", map[string]string{AnnSource: SourceNone}, nil))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "block"}})
		Expect(err).ToNot(HaveOccurred())
		resultPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "block"}, resultPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resultPvc.GetAnnotations()[AnnPodPhase]).To(BeEquivalentTo(corev1.PodSucceeded))
	})

	It("should do nothing and not error, if a PVC that is completed is passed", func() {
		orgPvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodSucceeded)}, nil)
		orgPvc.TypeMeta.APIVersion = "v1"
		orgPvc.TypeMeta.Kind = "PersistentVolumeClaim"
		reconciler = createImportReconciler(orgPvc)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Namespace: orgPvc.Namespace, Name: orgPvc.Name}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(reflect.DeepEqual(orgPvc, resPvc)).To(BeTrue())
	})

	It("Should init PVC with a POD name if a PVC with all needed annotations is passed", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint}, nil)
		pvc.Status.Phase = v1.ClaimBound
		reconciler = createImportReconciler(pvc)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		resultPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resultPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resultPvc.GetAnnotations()[AnnImportPod]).ToNot(BeEmpty())
	})

	It("Should requeue and not create a pod if target pvc in use", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnImportPod: "importer-testPvc1"}, nil)
		pvc.Status.Phase = v1.ClaimBound
		pod := podUsingPVC(pvc, false)
		reconciler := createImportReconciler(pvc, pod)
		result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeTrue())
		podList := &corev1.PodList{}
		err = reconciler.client.List(context.TODO(), podList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(podList.Items)).To(Equal(1))
		By("Checking events recorded")
		close(reconciler.recorder.(*record.FakeRecorder).Events)
		found := false
		for event := range reconciler.recorder.(*record.FakeRecorder).Events {
			By(fmt.Sprintf("Event: %v", event))

			if strings.Contains(event, "ImportTargetInUse") {
				found = true
			}
		}
		Expect(found).To(BeTrue())
	})
	It("Should create a POD if target pvc no longer in use (pod using the PVC failed)", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnImportPod: "importer-testPvc1"}, nil)
		pvc.Status.Phase = v1.ClaimBound
		podFinishedUsingPvc := podUsingPVC(pvc, false)
		podFinishedUsingPvc.Status.Phase = v1.PodFailed
		reconciler := createImportReconciler(pvc, podFinishedUsingPvc)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		pod := &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, pod)
		Expect(err).ToNot(HaveOccurred())
		foundEndPoint := false
		for _, envVar := range pod.Spec.Containers[0].Env {
			if envVar.Name == common.ImporterEndpoint {
				foundEndPoint = true
				Expect(envVar.Value).To(Equal(testEndPoint))
			}
		}
		Expect(foundEndPoint).To(BeTrue())
		By("Verifying the fsGroup of the pod is the qemu user")
		Expect(*pod.Spec.SecurityContext.FSGroup).To(Equal(int64(107)))
	})

	It("Should create a POD with node placement", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnImportPod: "importer-testPvc1"}, nil)
		pvc.Status.Phase = v1.ClaimBound

		reconciler = createImportReconciler(pvc)

		cr := &cdiv1.CDI{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "cdi"}, cr)
		Expect(err).ToNot(HaveOccurred())

		dummyNodeSelector := map[string]string{"kubernetes.io/arch": "amd64"}
		dummyTolerations := []v1.Toleration{{Key: "test", Value: "123"}}
		dummyAffinity := &v1.Affinity{
			NodeAffinity: &v1.NodeAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
					NodeSelectorTerms: []v1.NodeSelectorTerm{
						{
							MatchExpressions: []v1.NodeSelectorRequirement{
								{Key: "kubernetes.io/hostname", Operator: v1.NodeSelectorOpIn, Values: []string{"node01"}},
							},
						},
					},
				},
			},
		}
		cr.Spec.Workloads.NodeSelector = dummyNodeSelector
		cr.Spec.Workloads.Affinity = dummyAffinity
		cr.Spec.Workloads.Tolerations = dummyTolerations

		err = reconciler.client.Update(context.TODO(), cr)
		Expect(err).ToNot(HaveOccurred())

		placement, err := GetWorkloadNodePlacement(reconciler.client)
		Expect(err).ToNot(HaveOccurred())

		Expect(placement.Affinity).To(Equal(dummyAffinity))
		Expect(placement.NodeSelector).To(Equal(dummyNodeSelector))
		Expect(placement.Tolerations).To(Equal(dummyTolerations))

		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		pod := &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.Affinity).To(Equal(dummyAffinity))
		Expect(pod.Spec.NodeSelector).To(Equal(dummyNodeSelector))
		Expect(pod.Spec.Tolerations).To(Equal(dummyTolerations))
	})

	It("Should create a POD if a PVC with all needed annotations is passed", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnImportPod: "importer-testPvc1", AnnPodNetwork: "net1"}, nil)
		pvc.Status.Phase = v1.ClaimBound
		reconciler = createImportReconciler(pvc)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		pod := &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, pod)
		Expect(err).ToNot(HaveOccurred())
		foundEndPoint := false
		for _, envVar := range pod.Spec.Containers[0].Env {
			if envVar.Name == common.ImporterEndpoint {
				foundEndPoint = true
				Expect(envVar.Value).To(Equal(testEndPoint))
			}
		}
		Expect(foundEndPoint).To(BeTrue())
		By("Verifying the fsGroup of the pod is the qemu user")
		Expect(*pod.Spec.SecurityContext.FSGroup).To(Equal(int64(107)))
		By("Verifying the pod is annotated correctly")
		Expect(pod.GetAnnotations()[AnnPodNetwork]).To(Equal("net1"))
		Expect(pod.GetAnnotations()[AnnPodSidecarInjection]).To(Equal(AnnPodSidecarInjectionDefault))
	})

	It("Should not pass non-approved PVC annotation to created POD", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnImportPod: "importer-testPvc1", "annot1": "value1"}, nil)
		pvc.Status.Phase = v1.ClaimBound
		reconciler = createImportReconciler(pvc)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		pod := &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, pod)
		Expect(err).ToNot(HaveOccurred())
		foundEndPoint := false
		for _, envVar := range pod.Spec.Containers[0].Env {
			if envVar.Name == common.ImporterEndpoint {
				foundEndPoint = true
				Expect(envVar.Value).To(Equal(testEndPoint))
			}
		}
		Expect(foundEndPoint).To(BeTrue())
		By("Verifying the pod is not annotated with annot")
		Expect(pod.GetAnnotations()["annot1"]).ToNot(Equal("value1"))
	})

	It("Should create a POD if a bound PVC with all needed annotations is passed, but not set fsgroup if not kubevirt contenttype", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnImportPod: "importer-testPvc1", AnnContentType: string(cdiv1.DataVolumeArchive)}, nil)
		pvc.Status.Phase = v1.ClaimBound
		reconciler = createImportReconciler(pvc)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		pod := &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, pod)
		Expect(err).ToNot(HaveOccurred())
		foundEndPoint := false
		for _, envVar := range pod.Spec.Containers[0].Env {
			if envVar.Name == common.ImporterEndpoint {
				foundEndPoint = true
				Expect(envVar.Value).To(Equal(testEndPoint))
			}
		}
		Expect(foundEndPoint).To(BeTrue())
		By("Verifying the fsGroupis not set")
		Expect(pod.Spec.SecurityContext).To(BeNil())
	})

	It("Should error if a POD with the same name exists, but is not owned by the PVC, if a PVC with all needed annotations is passed", func() {
		pod := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "importer-testPvc1",
				Namespace: "default",
			},
		}
		reconciler = createImportReconciler(createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint}, nil), pod)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Pod is not owned by PVC"))
	})
})

var _ = Describe("Update PVC from POD", func() {
	var (
		reconciler *ImportReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			reconciler = nil
		}
	})

	It("Should update the PVC status to succeeded, if pod is succeeded and then delete the pod", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending)}, nil)
		pod := createImporterTestPod(pvc, "testPvc1", nil)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Message: "Import Completed",
							Reason:  "Reason",
						},
					},
				},
			},
		}
		reconciler = createImportReconciler(pvc, pod)
		resPod := &corev1.Pod{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.updatePvcFromPod(pvc, pod, reconciler.log)
		Expect(err).ToNot(HaveOccurred())
		By("Checking import successful event recorded")
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring("Import Successful"))
		By("Checking pvc phase has been updated")
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[AnnPodPhase]).To(BeEquivalentTo(corev1.PodSucceeded))
		By("Checking pod has been deleted")
		resPod = &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
		Expect(resPvc.GetAnnotations()[AnnRunningCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[AnnRunningConditionMessage]).To(Equal("Import Completed"))
		Expect(resPvc.GetAnnotations()[AnnRunningConditionReason]).To(Equal("Reason"))
	})

	It("Should update the PVC status to running, if pod is running", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending)}, nil)
		pod := createImporterTestPod(pvc, "testPvc1", nil)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Running: &v1.ContainerStateRunning{},
					},
				},
			},
		}
		reconciler = createImportReconciler(pvc, pod)
		resPod := &corev1.Pod{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.updatePvcFromPod(pvc, pod, reconciler.log)
		Expect(err).ToNot(HaveOccurred())
		By("Checking pvc phase has been updated")
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[AnnPodPhase]).To(BeEquivalentTo(corev1.PodRunning))
		Expect(resPvc.GetAnnotations()[AnnImportPod]).To(Equal(pod.Name))
		By("Checking pod has NOT been deleted")
		resPod = &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).ToNot(HaveOccurred())
		By("Making sure the label has been added")
		Expect(resPvc.GetLabels()[common.CDILabelKey]).To(Equal(common.CDILabelValue))
		Expect(resPvc.GetAnnotations()[AnnRunningCondition]).To(Equal("true"))
		Expect(resPvc.GetAnnotations()[AnnRunningConditionMessage]).To(Equal(""))
		Expect(resPvc.GetAnnotations()[AnnRunningConditionReason]).To(Equal("Pod is running"))
	})

	It("Should create scratch PVC, if pod is pending and PVC is marked with scratch", func() {
		scratchPvcName := &corev1.PersistentVolumeClaim{}
		scratchPvcName.Name = "testPvc1-scratch"
		pvc := createPvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending), AnnRequiresScratch: "true"}, nil, corev1.ClaimBound)
		pod := createImporterTestPod(pvc, "testPvc1", scratchPvcName)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Waiting: &v1.ContainerStateWaiting{
							Message: "Pending",
						},
					},
				},
			},
		}
		reconciler = createImportReconciler(pvc, pod)
		err := reconciler.updatePvcFromPod(pvc, pod, reconciler.log)
		Expect(err).ToNot(HaveOccurred())
		By("Checking scratch PVC has been created")
		// Once all controllers are converted, we will use the runtime lib client instead of client-go and retrieval needs to change here.
		scratchPvc := &v1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1-scratch", Namespace: "default"}, scratchPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(scratchPvc.Spec.Resources).To(Equal(pvc.Spec.Resources))

		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[AnnImportPod]).To(Equal(pod.Name))
		Expect(resPvc.GetAnnotations()[AnnRunningCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[AnnRunningConditionMessage]).To(Equal("Pending"))
		Expect(resPvc.GetAnnotations()[AnnRunningConditionReason]).To(BeEmpty())
		Expect(resPvc.GetAnnotations()[AnnBoundCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[AnnBoundConditionMessage]).To(Equal("Creating scratch space"))
		Expect(resPvc.GetAnnotations()[AnnBoundConditionReason]).To(Equal(creatingScratch))

	})

	// TODO: Update me to stay in progress if we were in progress already, its a pod failure and it will get restarted.
	It("Should update phase on PVC, if pod exited with error state that is NOT scratchspace exit", func() {
		pvc := createPvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodRunning)}, nil, corev1.ClaimBound)
		pod := createImporterTestPod(pvc, "testPvc1", nil)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodFailed,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					RestartCount: 2,
					State: v1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Message:  "I went poof",
							Reason:   "Explosion",
						},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Message:  "I went poof",
							Reason:   "Explosion",
						},
					},
				},
			},
		}
		reconciler = createImportReconciler(pvc, pod)
		err := reconciler.updatePvcFromPod(pvc, pod, reconciler.log)
		Expect(err).ToNot(HaveOccurred())
		By("Checking pvc phase has been updated")
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[AnnPodPhase]).To(BeEquivalentTo(corev1.PodFailed))
		Expect(resPvc.GetAnnotations()[AnnImportPod]).To(Equal(pod.Name))
		Expect(resPvc.GetAnnotations()[AnnPodRestarts]).To(Equal("2"))
		By("Checking error event recorded")
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring("I went poof"))
		Expect(resPvc.GetAnnotations()[AnnRunningCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[AnnRunningConditionMessage]).To(Equal("I went poof"))
		Expect(resPvc.GetAnnotations()[AnnRunningConditionReason]).To(Equal("Explosion"))
	})

	It("Should NOT update phase on PVC, if pod exited with error state that is scratchspace exit", func() {
		pvc := createPvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodRunning)}, nil, corev1.ClaimBound)
		scratchPvcName := &corev1.PersistentVolumeClaim{}
		scratchPvcName.Name = "testPvc1-scratch"
		pod := createImporterTestPod(pvc, "testPvc1", scratchPvcName)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: common.ScratchSpaceNeededExitCode,
							Message:  "scratch space needed",
						},
					},
					State: v1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Message:  "I went poof",
							Reason:   "Explosion",
						},
					},
				},
			},
		}
		reconciler = createImportReconciler(pvc, pod)
		err := reconciler.updatePvcFromPod(pvc, pod, reconciler.log)
		Expect(err).ToNot(HaveOccurred())
		By("Checking pvc phase has been updated")
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		By("Verifying that the phase hasn't changed")
		Expect(resPvc.GetAnnotations()[AnnPodPhase]).To(BeEquivalentTo(corev1.PodRunning))
		Expect(resPvc.GetAnnotations()[AnnImportPod]).To(Equal(pod.Name))
		Expect(resPvc.GetAnnotations()[AnnPodRestarts]).To(Equal("0"))
		// No scratch space because the pod is not in pending.
		Expect(resPvc.GetAnnotations()[AnnBoundCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[AnnBoundConditionMessage]).To(Equal("Creating scratch space"))
		Expect(resPvc.GetAnnotations()[AnnBoundConditionReason]).To(Equal(creatingScratch))
		Expect(resPvc.GetAnnotations()[AnnRunningCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[AnnRunningConditionMessage]).To(Equal("I went poof"))
		Expect(resPvc.GetAnnotations()[AnnRunningConditionReason]).To(Equal("Explosion"))
	})

	It("Should mark PVC as waiting for VDDK configmap, if not already present", func() {
		pvc := createPvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{AnnEndpoint: testEndPoint, AnnImportPod: "testpod", AnnSource: SourceVDDK}, nil, corev1.ClaimPending)
		reconciler = createImportReconciler(pvc)
		err := reconciler.createImporterPod(pvc)
		By("Checking importer pod creation returned an error")
		Expect(err).To(HaveOccurred())
		By("Checking pvc annotations have been updated")
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[AnnBoundCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[AnnBoundConditionMessage]).To(Equal("waiting for v2v-vmware configmap for VDDK image"))
		Expect(resPvc.GetAnnotations()[AnnBoundConditionReason]).To(Equal(common.AwaitingVDDK))

		By("Checking again after creating configmap")
		configmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.VddkConfigMap,
				Namespace: "cdi",
			},
			Data: map[string]string{
				common.VddkConfigDataKey: "test",
			},
		}
		reconciler.client.Create(context.TODO(), configmap)
		err = reconciler.createImporterPod(pvc)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should not mark PVC as waiting for VDDK configmap, if already present", func() {
		configmap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      common.VddkConfigMap,
				Namespace: "cdi",
			},
			Data: map[string]string{
				common.VddkConfigDataKey: "test",
			},
		}
		pvc := createPvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{AnnEndpoint: testEndPoint, AnnImportPod: "testpod", AnnSource: SourceVDDK}, nil, corev1.ClaimBound)
		reconciler = createImportReconciler(configmap, pvc)
		err := reconciler.createImporterPod(pvc)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should copy VDDK connection information to annotations on PVC", func() {
		pvc := createPvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodRunning), AnnSource: SourceVDDK}, nil, corev1.ClaimBound)
		scratchPvcName := &corev1.PersistentVolumeClaim{}
		scratchPvcName.Name = "testPvc1-scratch"
		pod := createImporterTestPod(pvc, "testPvc1", scratchPvcName)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
							Message:  "",
						},
					},
					State: v1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
							Message:  `Import Complete; VDDK: {"Version": "1.0.0", "Host": "esx15.test.lan"}`,
							Reason:   "Completed",
						},
					},
				},
			},
		}
		reconciler = createImportReconciler(pvc, pod)
		err := reconciler.updatePvcFromPod(pvc, pod, reconciler.log)
		Expect(err).ToNot(HaveOccurred())
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[AnnVddkHostConnection]).To(Equal("esx15.test.lan"))
		Expect(resPvc.GetAnnotations()[AnnVddkVersion]).To(Equal("1.0.0"))
	})

})

var _ = Describe("Create Importer Pod", func() {
	var scratchPvcName = "scratchPvc"

	table.DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, scratchPvcName *string) {
		reconciler := createImportReconciler(pvc)
		podEnvVar := &importPodEnvVar{
			ep:                 "",
			httpProxy:          "",
			httpsProxy:         "",
			secretName:         "",
			source:             "",
			contentType:        "",
			imageSize:          "1G",
			certConfigMap:      "",
			diskID:             "",
			filesystemOverhead: "0.055",
			insecureTLS:        false,
		}
		pod, err := createImporterPod(reconciler.log, reconciler.client, testImage, "5", testPullPolicy, podEnvVar, pvc, scratchPvcName, nil, pvc.Annotations[AnnPriorityClassName])
		Expect(err).ToNot(HaveOccurred())
		By("Verifying PVC owns pod")
		Expect(len(pod.GetOwnerReferences())).To(Equal(1))
		Expect(pod.GetOwnerReferences()[0].UID).To(Equal(pvc.GetUID()))
		By("Verifying volume mode is correct")
		if getVolumeMode(pvc) == corev1.PersistentVolumeBlock {
			Expect(pod.Spec.Containers[0].VolumeDevices[0].Name).To(Equal(DataVolName))
			Expect(pod.Spec.Containers[0].VolumeDevices[0].DevicePath).To(Equal(common.WriteBlockPath))
			Expect(pod.Spec.SecurityContext.RunAsUser).To(Equal(&[]int64{0}[0]))
			if scratchPvcName != nil {
				By("Verifying scratch space is set if available")
				Expect(len(pod.Spec.Containers[0].VolumeMounts)).To(Equal(1))
				Expect(pod.Spec.Containers[0].VolumeMounts[0].Name).To(Equal(ScratchVolName))
				Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal(common.ScratchDataDir))
			}
		} else {
			Expect(pod.Spec.Containers[0].VolumeMounts[0].Name).To(Equal(DataVolName))
			Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal(common.ImporterDataDir))
			if scratchPvcName != nil {
				By("Verifying scratch space is set if available")
				Expect(len(pod.Spec.Containers[0].VolumeMounts)).To(Equal(2))
				Expect(pod.Spec.Containers[0].VolumeMounts[1].Name).To(Equal(ScratchVolName))
				Expect(pod.Spec.Containers[0].VolumeMounts[1].MountPath).To(Equal(common.ScratchDataDir))
			}
		}
		By("Verifying container spec is correct")
		Expect(pod.Spec.Containers[0].Image).To(Equal(testImage))
		Expect(pod.Spec.Containers[0].ImagePullPolicy).To(BeEquivalentTo(testPullPolicy))
		Expect(pod.Spec.Containers[0].Args[0]).To(Equal("-v=5"))
		Expect(pod.Spec.PriorityClassName).To(Equal(pvc.Annotations[AnnPriorityClassName]))
	},
		table.Entry("should create pod with file system volume mode", createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending), AnnImportPod: "podName", AnnPriorityClassName: "p0"}, nil), nil),
		table.Entry("should create pod with block volume mode", createBlockPvc("testBlockPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending), AnnImportPod: "podName", AnnPriorityClassName: "p0"}, nil), nil),
		table.Entry("should create pod with file system volume mode and scratchspace", createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending), AnnImportPod: "podName", AnnPriorityClassName: "p0"}, nil), &scratchPvcName),
		table.Entry("should create pod with block volume mode and scratchspace", createBlockPvc("testBlockPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending), AnnImportPod: "podName", AnnPriorityClassName: "p0"}, nil), &scratchPvcName),
	)
})

var _ = Describe("Import test env", func() {
	const mockUID = "1111-1111-1111-1111"

	It("Should create import env", func() {
		testEnvVar := &importPodEnvVar{
			ep:                 "myendpoint",
			httpProxy:          "httpproxy",
			httpsProxy:         "httpsproxy",
			noProxy:            "httpproxy",
			secretName:         "",
			source:             SourceHTTP,
			contentType:        string(cdiv1.DataVolumeKubeVirt),
			imageSize:          "1G",
			certConfigMap:      "",
			diskID:             "",
			uuid:               "",
			backingFile:        "",
			thumbprint:         "",
			filesystemOverhead: "0.055",
			insecureTLS:        false,
			currentCheckpoint:  "",
			previousCheckpoint: "",
			finalCheckpoint:    "",
			preallocation:      false}
		Expect(reflect.DeepEqual(makeImportEnv(testEnvVar, mockUID), createImportTestEnv(testEnvVar, mockUID))).To(BeTrue())
	})
})

var _ = Describe("getSecretName", func() {
	It("should find a secret", func() {
		pvcWithAnno := createPvc("testPVCWithAnno", "default", map[string]string{AnnSecret: "mysecret"}, nil)
		testSecret := createSecret("mysecret", "default", "mysecretkey", "mysecretstring", map[string]string{AnnSecret: "mysecret"})
		reconciler := createImportReconciler(pvcWithAnno, testSecret)
		result := reconciler.getSecretName(pvcWithAnno)
		Expect(result).To(Equal("mysecret"))
	})

	It("should not find a secret", func() {
		pvcNoAnno := createPvc("testPVCNoAnno", "default", nil, nil)
		testSecret := createSecret("mysecret2", "default", "mysecretkey2", "mysecretstring2", map[string]string{AnnSecret: "mysecret2"})
		reconciler := createImportReconciler(pvcNoAnno, testSecret)
		result := reconciler.getSecretName(pvcNoAnno)
		Expect(result).To(Equal(""))
	})
})

var _ = Describe("getCertConfigMap", func() {
	testConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configMapName",
			Namespace: "default",
		},
	}

	It("should find the configmap if PVC has one defined, and it exists", func() {
		pvcWithAnno := createPvc("testPVCWithAnno", "default", map[string]string{AnnCertConfigMap: "configMapName"}, nil)
		reconciler := createImportReconciler(pvcWithAnno, testConfigMap)
		cdiConfig := &cdiv1.CDIConfig{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		cm, err := reconciler.getCertConfigMap(pvcWithAnno)
		Expect(err).ToNot(HaveOccurred())
		Expect(cm).To(Equal(testConfigMap.Name))
	})

	It("should return the configmap name if PVC has one defined, but doesn't exist", func() {
		pvcWithAnno := createPvc("testPVCWithAnno", "default", map[string]string{AnnCertConfigMap: "doesnotexist"}, nil)
		reconciler := createImportReconciler(pvcWithAnno)
		cdiConfig := &cdiv1.CDIConfig{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		cm, err := reconciler.getCertConfigMap(pvcWithAnno)
		Expect(err).ToNot(HaveOccurred())
		Expect(cm).To(Equal("doesnotexist"))
	})

	It("should return blank if the pvc has no annotation", func() {
		pvcNoAnno := createPvc("testPVC", "default", nil, nil)
		reconciler := createImportReconciler(pvcNoAnno)
		cdiConfig := &cdiv1.CDIConfig{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		cm, err := reconciler.getCertConfigMap(pvcNoAnno)
		Expect(err).ToNot(HaveOccurred())
		Expect(cm).To(Equal(""))
	})
})

var _ = Describe("getInsecureTLS", func() {
	host := "myregistry"
	endpointNoPort := "docker://" + host
	hostWithPort := host + ":5000"
	endpointWithPort := "docker://" + hostWithPort

	table.DescribeTable("should", func(endpoint string, insecureHost string, isInsecure bool) {
		pvc := createPvc("testPVC", "default", map[string]string{AnnEndpoint: endpoint}, nil)
		reconciler := createImportReconciler(pvc)

		cdiConfig := &cdiv1.CDIConfig{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())

		if insecureHost != "" {
			cdiConfig.Spec.InsecureRegistries = []string{insecureHost}
		}

		result, err := reconciler.isInsecureTLS(pvc, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(isInsecure))
	},
		table.Entry("return true on endpoint with no port, and host defined", endpointNoPort, host, true),
		table.Entry("return true on endpoint with port, and host with port", endpointWithPort, hostWithPort, true),
		table.Entry("return false on endpoint with no port, and host with port", endpointNoPort, hostWithPort, false),
		table.Entry("return false on endpoint with port, and host defined", endpointWithPort, host, false),
		table.Entry("return false on endpoint with no port, and blank host", endpointNoPort, "", false),
		table.Entry("return false on blank endpoint, and host defined", "", host, false),
	)
})

var _ = Describe("GetContentType", func() {
	pvcNoAnno := createPvc("testPVCNoAnno", "default", nil, nil)
	pvcArchiveAnno := createPvc("testPVCArchiveAnno", "default", map[string]string{AnnContentType: string(cdiv1.DataVolumeArchive)}, nil)
	pvcKubevirtAnno := createPvc("testPVCKubevirtAnno", "default", map[string]string{AnnContentType: string(cdiv1.DataVolumeKubeVirt)}, nil)
	pvcInvalidValue := createPvc("testPVCInvalidValue", "default", map[string]string{AnnContentType: "iaminvalid"}, nil)

	table.DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, expectedResult cdiv1.DataVolumeContentType) {
		result := GetContentType(pvc)
		Expect(result).To(BeEquivalentTo(expectedResult))
	},
		table.Entry("return kubevirt contenttype if no annotation provided", pvcNoAnno, cdiv1.DataVolumeKubeVirt),
		table.Entry("return archive contenttype if archive annotation present", pvcArchiveAnno, cdiv1.DataVolumeArchive),
		table.Entry("return kubevirt contenttype if kubevirt annotation present", pvcKubevirtAnno, cdiv1.DataVolumeKubeVirt),
		table.Entry("return kubevirt contenttype if invalid annotation provided", pvcInvalidValue, cdiv1.DataVolumeKubeVirt),
	)
})

var _ = Describe("getSource", func() {
	pvcNoAnno := createPvc("testPVCNoAnno", "default", nil, nil)
	pvcNoneAnno := createPvc("testPVCNoneAnno", "default", map[string]string{AnnSource: SourceNone}, nil)
	pvcGlanceAnno := createPvc("testPVCNoneAnno", "default", map[string]string{AnnSource: SourceGlance}, nil)
	pvcInvalidValue := createPvc("testPVCInvalidValue", "default", map[string]string{AnnSource: "iaminvalid"}, nil)
	pvcRegistryAnno := createPvc("testPVCRegistryAnno", "default", map[string]string{AnnSource: SourceRegistry}, nil)
	pvcImageIOAnno := createPvc("testPVCImageIOAnno", "default", map[string]string{AnnSource: SourceImageio}, nil)
	pvcVDDKAnno := createPvc("testPVCVDDKAnno", "default", map[string]string{AnnSource: SourceVDDK}, nil)

	table.DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, expectedResult string) {
		result := getSource(pvc)
		Expect(result).To(BeEquivalentTo(expectedResult))
	},
		table.Entry("return none if none annotation provided", pvcNoneAnno, SourceNone),
		table.Entry("return http if no annotation provided", pvcNoAnno, SourceHTTP),
		table.Entry("return glance if glance annotation provided", pvcGlanceAnno, SourceGlance),
		table.Entry("return http if invalid annotation provided", pvcInvalidValue, SourceHTTP),
		table.Entry("return registry if registry annotation provided", pvcRegistryAnno, SourceRegistry),
		table.Entry("return imageio if imageio annotation provided", pvcImageIOAnno, SourceImageio),
		table.Entry("return vddk if vddk annotation provided", pvcVDDKAnno, SourceVDDK),
	)
})

var _ = Describe("getEndpoint", func() {
	pvcNoAnno := createPvc("testPVCNoAnno", "default", nil, nil)
	pvcWithAnno := createPvc("testPVCWithAnno", "default", map[string]string{AnnEndpoint: "http://test"}, nil)
	pvcNoValue := createPvc("testPVCNoValue", "default", map[string]string{AnnEndpoint: ""}, nil)

	table.DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, expectedResult string, expectErr bool) {
		result, err := getEndpoint(pvc)
		Expect(result).To(BeEquivalentTo(expectedResult))
		if expectErr {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
		}
	},
		table.Entry("return blank and error if no annotation provided", pvcNoAnno, "", true),
		table.Entry("return value and no error if valid annotation provided", pvcWithAnno, "http://test", false),
		table.Entry("return blank and error if blank annotation provided", pvcNoValue, "", true),
	)
})

func createImportReconciler(objects ...runtime.Object) *ImportReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register cdi types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)

	objs = append(objs, MakeEmptyCDICR())

	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	objs = append(objs, cdiConfig)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	// Increase this if you have more than one event that fires.
	rec := record.NewFakeRecorder(1)
	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &ImportReconciler{
		client:         cl,
		uncachedClient: cl,
		scheme:         s,
		log:            importLog,
		recorder:       rec,
		featureGates:   featuregates.NewFeatureGates(cl),
	}
	return r
}

func createFeatureGates() featuregates.FeatureGates {
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	cl := fake.NewFakeClientWithScheme(s, MakeEmptyCDIConfigSpec(common.ConfigName))
	return featuregates.NewFeatureGates(cl)
}

func createImportTestEnv(podEnvVar *importPodEnvVar, uid string) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{
			Name:  common.ImporterSource,
			Value: podEnvVar.source,
		},
		{
			Name:  common.ImporterEndpoint,
			Value: podEnvVar.ep,
		},
		{
			Name:  common.ImporterContentType,
			Value: podEnvVar.contentType,
		},
		{
			Name:  common.ImporterImageSize,
			Value: podEnvVar.imageSize,
		},
		{
			Name:  common.OwnerUID,
			Value: string(uid),
		},
		{
			Name:  common.FilesystemOverheadVar,
			Value: podEnvVar.filesystemOverhead,
		},
		{
			Name:  common.InsecureTLSVar,
			Value: strconv.FormatBool(podEnvVar.insecureTLS),
		},
		{
			Name:  common.ImporterDiskID,
			Value: podEnvVar.diskID,
		},
		{
			Name:  common.ImporterUUID,
			Value: podEnvVar.uuid,
		},
		{
			Name:  common.ImporterBackingFile,
			Value: podEnvVar.backingFile,
		},
		{
			Name:  common.ImporterThumbprint,
			Value: podEnvVar.thumbprint,
		},
		{
			Name:  common.ImportProxyHTTP,
			Value: podEnvVar.httpProxy,
		},
		{
			Name:  common.ImportProxyHTTPS,
			Value: podEnvVar.httpsProxy,
		},
		{
			Name:  common.ImportProxyNoProxy,
			Value: podEnvVar.noProxy,
		},
		{
			Name:  common.ImporterCurrentCheckpoint,
			Value: podEnvVar.currentCheckpoint,
		},
		{
			Name:  common.ImporterPreviousCheckpoint,
			Value: podEnvVar.previousCheckpoint,
		},
		{
			Name:  common.ImporterFinalCheckpoint,
			Value: podEnvVar.finalCheckpoint,
		},
		{
			Name:  common.Preallocation,
			Value: strconv.FormatBool(podEnvVar.preallocation),
		},
	}

	if podEnvVar.secretName != "" {
		env = append(env, corev1.EnvVar{
			Name: common.ImporterAccessKeyID,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: podEnvVar.secretName,
					},
					Key: common.KeyAccess,
				},
			},
		}, corev1.EnvVar{
			Name: common.ImporterSecretKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: podEnvVar.secretName,
					},
					Key: common.KeySecret,
				},
			},
		})
	}
	return env
}

func createImporterTestPod(pvc *corev1.PersistentVolumeClaim, dvname string, scratchPvc *corev1.PersistentVolumeClaim) *corev1.Pod {
	// importer pod name contains the pvc name
	podName := fmt.Sprintf("%s-%s", common.ImporterPodName, pvc.Name)

	blockOwnerDeletion := true
	isController := true

	volumes := []corev1.Volume{
		{
			Name: dvname,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvc.Name,
					ReadOnly:  false,
				},
			},
		},
	}

	if scratchPvc != nil {
		volumes = append(volumes, corev1.Volume{
			Name: ScratchVolName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: scratchPvc.Name,
					ReadOnly:  false,
				},
			},
		})
	}

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: pvc.Namespace,
			Annotations: map[string]string{
				AnnCreatedBy: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: common.ImporterPodName,
				LabelImportPvc:           pvc.Name,
				common.PrometheusLabel:   "",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "PersistentVolumeClaim",
					Name:               pvc.Name,
					UID:                pvc.GetUID(),
					BlockOwnerDeletion: &blockOwnerDeletion,
					Controller:         &isController,
				},
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            common.ImporterPodName,
					Image:           "test/myimage",
					ImagePullPolicy: corev1.PullPolicy("Always"),
					Args:            []string{"-v=5"},
					Ports: []corev1.ContainerPort{
						{
							Name:          "metrics",
							ContainerPort: 8443,
							Protocol:      corev1.ProtocolTCP,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Volumes:       volumes,
		},
	}

	ep, _ := getEndpoint(pvc)
	source := getSource(pvc)
	contentType := GetContentType(pvc)
	imageSize, _ := getRequestedImageSize(pvc)
	volumeMode := getVolumeMode(pvc)

	env := []corev1.EnvVar{
		{
			Name:  common.ImporterSource,
			Value: source,
		},
		{
			Name:  common.ImporterEndpoint,
			Value: ep,
		},
		{
			Name:  common.ImporterContentType,
			Value: contentType,
		},
		{
			Name:  common.ImporterImageSize,
			Value: imageSize,
		},
		{
			Name:  common.OwnerUID,
			Value: string(pvc.UID),
		},
		{
			Name:  common.InsecureTLSVar,
			Value: "false",
		},
	}
	pod.Spec.Containers[0].Env = env
	if volumeMode == corev1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = addVolumeDevices()
		pod.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser: &[]int64{0}[0],
		}
	} else {
		pod.Spec.Containers[0].VolumeMounts = addImportVolumeMounts()
	}

	if scratchPvc != nil {
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      ScratchVolName,
			MountPath: common.ScratchDataDir,
		})
	}

	return pod
}

type FakeFeatureGates struct {
	honorWaitForFirstConsumerEnabled bool
}

func (f *FakeFeatureGates) HonorWaitForFirstConsumerEnabled() (bool, error) {
	return f.honorWaitForFirstConsumerEnabled, nil
}
