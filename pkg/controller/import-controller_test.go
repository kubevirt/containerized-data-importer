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
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
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
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnPodPhase: string(corev1.PodSucceeded)}, nil)
		Expect(cc.IsPVCComplete(testPvc)).To(BeTrue())
	})

	It("Should NOT return complete if annotation is not succeeded", func() {
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnPodPhase: string(corev1.PodPending)}, nil)
		Expect(cc.IsPVCComplete(testPvc)).To(BeFalse())
	})

	It("Should NOT return complete if annotation is missing", func() {
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{}, nil)
		Expect(cc.IsPVCComplete(testPvc)).To(BeFalse())
	})

	It("Should be interesting if NOT complete, and endpoint and source is set", func() {
		r := createImportReconciler()
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnPodPhase: string(corev1.PodPending), cc.AnnEndpoint: testEndPoint, cc.AnnSource: cc.SourceHTTP}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeTrue())
	})

	It("Should NOT be interesting if complete, and endpoint and source is set", func() {
		r := createImportReconciler()
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnPodPhase: string(corev1.PodSucceeded), cc.AnnEndpoint: testEndPoint, cc.AnnSource: cc.SourceHTTP}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeFalse())
	})

	It("Should be interesting if NOT complete, and endpoint missing and source is set", func() {
		r := createImportReconciler()
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnPodPhase: string(corev1.PodRunning), cc.AnnSource: cc.SourceHTTP}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeTrue())
	})

	It("Should be interesting if NOT complete, and endpoint set and source is missing", func() {
		r := createImportReconciler()
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnPodPhase: string(corev1.PodPending), cc.AnnEndpoint: testEndPoint}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeTrue())
	})

	It("Should NOT be interesting if NOT BOUND, and endpoint and source is set, and honorWaitForFirstConsumerEnabled", func() {
		r := createImportReconciler()
		r.featureGates = &FakeFeatureGates{honorWaitForFirstConsumerEnabled: true}
		testPvc := createPendingPvc("testPvc1", "default", map[string]string{cc.AnnPodPhase: string(corev1.PodPending), cc.AnnEndpoint: testEndPoint, cc.AnnSource: cc.SourceHTTP}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeFalse())
	})

	It("Should be interesting if NOT BOUND, and endpoint and source is set, and honorWaitForFirstConsumerEnabled and isImmediateBindingRequested is requested", func() {
		r := createImportReconciler()
		r.featureGates = &FakeFeatureGates{honorWaitForFirstConsumerEnabled: true}
		testPvc := createPendingPvc("testPvc1", "default", map[string]string{
			cc.AnnPodPhase:         string(corev1.PodPending),
			cc.AnnEndpoint:         testEndPoint,
			cc.AnnSource:           cc.SourceHTTP,
			cc.AnnImmediateBinding: "true",
		}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeTrue())
	})
	It("Should be interesting if NOT BOUND, and endpoint and source is set, and honorWaitForFirstConsumerEnabled is false and isImmediateBindingRequested is requested", func() {
		r := createImportReconciler()
		r.featureGates = &FakeFeatureGates{honorWaitForFirstConsumerEnabled: false}
		testPvc := createPendingPvc("testPvc1", "default", map[string]string{
			cc.AnnPodPhase:         string(corev1.PodPending),
			cc.AnnEndpoint:         testEndPoint,
			cc.AnnSource:           cc.SourceHTTP,
			cc.AnnImmediateBinding: "true",
		}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeTrue())
	})

	It("Should be interesting if complete, and endpoint and source is set, and multistage import not done", func() {
		r := createImportReconciler()
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{
			cc.AnnPodPhase: string(corev1.PodSucceeded),
			cc.AnnEndpoint: testEndPoint, cc.AnnSource: cc.SourceHTTP,
			cc.AnnCurrentCheckpoint: "test-check",
		}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeTrue())
	})

	It("Should NOT be interesting if complete, and endpoint and source is set, and multistage import done", func() {
		r := createImportReconciler()
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{
			cc.AnnPodPhase: string(corev1.PodSucceeded),
			cc.AnnEndpoint: testEndPoint, cc.AnnSource: cc.SourceHTTP,
			cc.AnnCurrentCheckpoint:    "test-check",
			cc.AnnMultiStageImportDone: "true",
		}, nil)
		Expect(r.shouldReconcilePVC(testPvc, importLog)).To(BeFalse())
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
		reconciler = createImportReconciler(cc.CreatePvc("testPvc1", "default", map[string]string{}, nil))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should return success if no PVC can be found, due to it not existing", func() {
		reconciler = createImportReconciler()
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should return success if no PVC can be found due to not existing in passed namespace", func() {
		reconciler = createImportReconciler(cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint}, nil))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "invalid"}})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should succeed and be marked complete, if creating a block PVC with source none", func() {
		pvc := createBlockPvc("testPvc1", "block", map[string]string{cc.AnnSource: cc.SourceNone}, nil)
		pod := cc.CreateImporterTestPod(pvc, "testPvc1", nil)
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
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "block"}})
		Expect(err).ToNot(HaveOccurred())
		resultPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "block"}, resultPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resultPvc.GetAnnotations()[cc.AnnPodPhase]).To(BeEquivalentTo(corev1.PodSucceeded))
	})

	It("should do nothing and not error, if a PVC that is completed is passed", func() {
		orgPvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnPodPhase: string(corev1.PodSucceeded)}, nil)
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
		pvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint}, nil)
		pvc.Status.Phase = v1.ClaimBound
		reconciler = createImportReconciler(pvc)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		resultPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resultPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resultPvc.GetAnnotations()[cc.AnnImportPod]).ToNot(BeEmpty())
	})

	It("Should requeue and not create a pod if target pvc in use", func() {
		pvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnImportPod: "importer-testPvc1"}, nil)
		pvc.Status.Phase = v1.ClaimBound
		pod := podUsingPVC(pvc, false)
		reconciler := createImportReconciler(pvc, pod)
		result, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeTrue())
		podList := &corev1.PodList{}
		err = reconciler.client.List(context.TODO(), podList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(podList.Items).To(HaveLen(1))
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
		pvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnImportPod: "importer-testPvc1"}, nil)
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
	})

	It("Should create a POD with node placement", func() {
		pvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnImportPod: "importer-testPvc1"}, nil)
		pvc.Status.Phase = v1.ClaimBound

		reconciler = createImportReconciler(pvc)
		workloads := updateCdiWithTestNodePlacement(reconciler.client)

		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		pod := &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, pod)
		Expect(err).ToNot(HaveOccurred())

		Expect(pod.Spec.Affinity).To(Equal(workloads.Affinity))
		Expect(pod.Spec.NodeSelector).To(Equal(workloads.NodeSelector))
		Expect(pod.Spec.Tolerations).To(Equal(workloads.Tolerations))
	})

	It("Should create a POD if a PVC with all needed annotations is passed", func() {
		pvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnImportPod: "importer-testPvc1", cc.AnnPodNetwork: "net1"}, nil)
		pvc.Status.Phase = v1.ClaimBound
		reconciler = createImportReconciler(pvc)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		pod := &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, pod)
		Expect(err).ToNot(HaveOccurred())
		Expect(pod.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))
		foundEndPoint := false
		for _, envVar := range pod.Spec.Containers[0].Env {
			if envVar.Name == common.ImporterEndpoint {
				foundEndPoint = true
				Expect(envVar.Value).To(Equal(testEndPoint))
			}
		}
		Expect(foundEndPoint).To(BeTrue())
		By("Verifying the pod is annotated correctly")
		Expect(pod.GetAnnotations()[cc.AnnPodNetwork]).To(Equal("net1"))
		Expect(pod.GetAnnotations()[cc.AnnPodSidecarInjectionIstio]).To(Equal(cc.AnnPodSidecarInjectionIstioDefault))
		Expect(pod.GetAnnotations()[cc.AnnPodSidecarInjectionLinkerd]).To(Equal(cc.AnnPodSidecarInjectionLinkerdDefault))
	})

	It("Should not pass non-approved PVC annotation to created POD", func() {
		pvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnImportPod: "importer-testPvc1", "annot1": "value1"}, nil)
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
		reconciler = createImportReconciler(cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint}, nil), pod)
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
		pvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnPodPhase: string(corev1.PodPending)}, nil)
		pod := cc.CreateImporterTestPod(pvc, "testPvc1", nil)
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
		Expect(resPvc.GetAnnotations()[cc.AnnPodPhase]).To(BeEquivalentTo(corev1.PodSucceeded))
		By("Checking pod has been deleted")
		resPod = &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
		Expect(resPvc.GetAnnotations()[cc.AnnRunningCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[cc.AnnRunningConditionMessage]).To(Equal("Import Completed"))
		Expect(resPvc.GetAnnotations()[cc.AnnRunningConditionReason]).To(Equal("Reason"))
	})

	DescribeTable("Should handle termination messages", func(termMsg, conditionMessage string) {
		pvc := cc.CreatePvc("testPvc1", "default", map[string]string{}, nil)
		pod := cc.CreateImporterTestPod(pvc, "testPvc1", nil)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Message: termMsg,
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

		Expect(resPvc.GetAnnotations()).To(HaveKeyWithValue(cc.AnnPodPhase, string(corev1.PodSucceeded)))
		Expect(resPvc.GetAnnotations()).To(HaveKeyWithValue(cc.AnnRunningCondition, "false"))
		Expect(resPvc.GetAnnotations()).To(HaveKeyWithValue(cc.AnnRunningConditionMessage, conditionMessage))
	},
		Entry("Message which can be unmarshalled", `{"preAllocationApplied": true, "message": "Import Complete"}`, "Import Complete"),
		Entry("Message which cannot be unmarshalled", "somemessage", "somemessage"),
	)

	DescribeTable("Update the PVC labels from termination message if pod is succeeded", func(phase v1.PodPhase, updated bool) {
		const testKeyExisting = "test"
		const testValueExisting = "existing"

		termMsg := common.TerminationMessage{
			Labels: map[string]string{
				"instancetype.kubevirt.io/default-instancetype": "u1.small",
				"instancetype.kubevirt.io/default-preference":   "fedora",
				testKeyExisting: "somethingelse",
			},
		}
		termMsgBytes, err := json.Marshal(termMsg)
		Expect(err).ToNot(HaveOccurred())

		// The existing key should not be overwritten
		pvc := cc.CreatePvc("testPvc1", "default", map[string]string{}, map[string]string{testKeyExisting: testValueExisting})
		pod := cc.CreateImporterTestPod(pvc, "testPvc1", nil)
		pod.Status = corev1.PodStatus{
			Phase: phase,
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Message: string(termMsgBytes),
						},
					},
				},
			},
		}
		reconciler = createImportReconciler(pvc, pod)
		err = reconciler.updatePvcFromPod(pvc, pod, reconciler.log)
		Expect(err).ToNot(HaveOccurred())

		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())

		for k, v := range termMsg.Labels {
			if k == testKeyExisting {
				Expect(resPvc.GetLabels()).To(HaveKeyWithValue(testKeyExisting, testValueExisting))
				continue
			}
			if updated {
				Expect(resPvc.GetLabels()).To(HaveKeyWithValue(k, v))
			} else {
				Expect(resPvc.GetLabels()).ToNot(HaveKey(k))
			}
		}
	},
		Entry("should", v1.PodSucceeded, true),
		Entry("should not", v1.PodFailed, false),
	)

	It("Should update the PVC status to running, if pod is running", func() {
		pvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnPodPhase: string(corev1.PodPending)}, nil)
		pod := cc.CreateImporterTestPod(pvc, "testPvc1", nil)
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
		Expect(resPvc.GetAnnotations()[cc.AnnPodPhase]).To(BeEquivalentTo(corev1.PodRunning))
		Expect(resPvc.GetAnnotations()[cc.AnnImportPod]).To(Equal(pod.Name))
		By("Checking pod has NOT been deleted")
		resPod = &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).ToNot(HaveOccurred())
		By("Making sure the label has been added")
		Expect(resPvc.GetLabels()[common.CDILabelKey]).To(Equal(common.CDILabelValue))
		Expect(resPvc.GetAnnotations()[cc.AnnRunningCondition]).To(Equal("true"))
		Expect(resPvc.GetAnnotations()[cc.AnnRunningConditionMessage]).To(Equal(""))
		Expect(resPvc.GetAnnotations()[cc.AnnRunningConditionReason]).To(Equal("Pod is running"))
	})

	It("Should create scratch PVC, if pod is pending and PVC is marked with scratch", func() {
		scratchPvc := &corev1.PersistentVolumeClaim{}
		scratchPvc.Name = "testPvc1-scratch"
		pvc := cc.CreatePvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnPodPhase: string(corev1.PodPending), cc.AnnRequiresScratch: "true"}, nil, corev1.ClaimBound)
		pod := cc.CreateImporterTestPod(pvc, "testPvc1", scratchPvc)
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
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1-scratch", Namespace: "default"}, scratchPvc)
		Expect(err).ToNot(HaveOccurred())
		// Since fsOverhead is 0, the scratch space size should be 1Mi aligned close to 1G
		requestSize := scratchPvc.Spec.Resources.Requests[corev1.ResourceStorage]
		Expect(requestSize.Value()).To(Equal(int64(999292928)))
		Expect(scratchPvc.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))

		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[cc.AnnImportPod]).To(Equal(pod.Name))
		Expect(resPvc.GetAnnotations()[cc.AnnRunningCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[cc.AnnRunningConditionMessage]).To(Equal("Pending"))
		Expect(resPvc.GetAnnotations()[cc.AnnRunningConditionReason]).To(BeEmpty())
		Expect(resPvc.GetAnnotations()[cc.AnnBoundCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[cc.AnnBoundConditionMessage]).To(Equal("Creating scratch space"))
		Expect(resPvc.GetAnnotations()[cc.AnnBoundConditionReason]).To(Equal(creatingScratch))

	})

	// TODO: Update me to stay in progress if we were in progress already, its a pod failure and it will get restarted.
	It("Should update phase on PVC, if pod exited with error state that is NOT scratchspace exit", func() {
		pvc := cc.CreatePvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnPodPhase: string(corev1.PodRunning)}, nil, corev1.ClaimBound)
		pod := cc.CreateImporterTestPod(pvc, "testPvc1", nil)
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
		Expect(resPvc.GetAnnotations()[cc.AnnPodPhase]).To(BeEquivalentTo(corev1.PodFailed))
		Expect(resPvc.GetAnnotations()[cc.AnnImportPod]).To(Equal(pod.Name))
		Expect(resPvc.GetAnnotations()[cc.AnnPodRestarts]).To(Equal("2"))
		By("Checking error event recorded")
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring("I went poof"))
		Expect(resPvc.GetAnnotations()[cc.AnnRunningCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[cc.AnnRunningConditionMessage]).To(Equal("I went poof"))
		Expect(resPvc.GetAnnotations()[cc.AnnRunningConditionReason]).To(Equal("Explosion"))
	})

	It("Should NOT update phase on PVC, if pod exited with termination message stating scratch space is required", func() {
		pvc := cc.CreatePvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnPodPhase: string(corev1.PodRunning)}, nil, corev1.ClaimBound)
		scratchPvc := &corev1.PersistentVolumeClaim{}
		scratchPvc.Name = "testPvc1-scratch"
		pod := cc.CreateImporterTestPod(pvc, "testPvc1", scratchPvc)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: v1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
							Message:  `{"scratchSpaceRequired": true}`,
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
		Expect(resPvc.GetAnnotations()[cc.AnnPodPhase]).To(BeEquivalentTo(corev1.PodRunning))
		Expect(resPvc.GetAnnotations()[cc.AnnImportPod]).To(Equal(pod.Name))
		Expect(resPvc.GetAnnotations()[cc.AnnPodRestarts]).To(Equal("0"))
		// No scratch space because the pod is not in pending.
		Expect(resPvc.GetAnnotations()[cc.AnnBoundCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[cc.AnnBoundConditionMessage]).To(Equal("Creating scratch space"))
		Expect(resPvc.GetAnnotations()[cc.AnnBoundConditionReason]).To(Equal(creatingScratch))
	})

	It("Should mark PVC as waiting for VDDK configmap, if not already present", func() {
		pvc := cc.CreatePvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnImportPod: "testpod", cc.AnnSource: cc.SourceVDDK}, nil, corev1.ClaimPending)
		reconciler = createImportReconciler(pvc)
		err := reconciler.createImporterPod(pvc)
		By("Checking importer pod creation returned an error")
		Expect(err).To(HaveOccurred())
		By("Checking pvc annotations have been updated")
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[cc.AnnBoundCondition]).To(Equal("false"))
		Expect(resPvc.GetAnnotations()[cc.AnnBoundConditionMessage]).To(Equal(fmt.Sprintf("waiting for v2v-vmware configmap or %s annotation for VDDK image", cc.AnnVddkInitImageURL)))
		Expect(resPvc.GetAnnotations()[cc.AnnBoundConditionReason]).To(Equal(common.AwaitingVDDK))

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
		Expect(reconciler.client.Create(context.TODO(), configmap)).To(Succeed())
		Expect(reconciler.createImporterPod(pvc)).To(Succeed())
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
		pvc := cc.CreatePvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnImportPod: "testpod", cc.AnnSource: cc.SourceVDDK}, nil, corev1.ClaimBound)
		reconciler = createImportReconciler(configmap, pvc)
		err := reconciler.createImporterPod(pvc)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should not mark PVC as waiting for VDDK configmap, if image URL annotation is present", func() {
		pvc := cc.CreatePvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnImportPod: "testpod", cc.AnnSource: cc.SourceVDDK, cc.AnnVddkInitImageURL: "test://image"}, nil, corev1.ClaimBound)
		reconciler = createImportReconciler(pvc)
		err := reconciler.createImporterPod(pvc)
		Expect(err).ToNot(HaveOccurred())
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[cc.AnnVddkInitImageURL]).To(Equal("test://image"))
		Expect(common.AwaitingVDDK).ToNot(Equal(resPvc.GetAnnotations()[cc.AnnBoundConditionReason]))
	})

	It("Should copy VDDK connection information to annotations on PVC", func() {
		pvc := cc.CreatePvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnPodPhase: string(corev1.PodRunning), cc.AnnSource: cc.SourceVDDK}, nil, corev1.ClaimBound)
		scratchPvc := &corev1.PersistentVolumeClaim{}
		scratchPvc.Name = "testPvc1-scratch"
		pod := cc.CreateImporterTestPod(pvc, "testPvc1", scratchPvc)
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
							Message:  `{"vddkInfo": {"Version": "1.0.0", "Host": "esx15.test.lan"}}`,
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
		Expect(resPvc.GetAnnotations()[cc.AnnVddkHostConnection]).To(Equal("esx15.test.lan"))
		Expect(resPvc.GetAnnotations()[cc.AnnVddkVersion]).To(Equal("1.0.0"))
	})

	It("Should delete pod for scratch space even if retainAfterCompletion is set", func() {
		annotations := map[string]string{
			cc.AnnEndpoint:  testEndPoint,
			cc.AnnImportPod: "testpod",
			// gets added by controller
			// cc.AnnRequiresScratch:          "true",
			cc.AnnSource:                   cc.SourceVDDK,
			cc.AnnPodRetainAfterCompletion: "true",
		}
		pvc := cc.CreatePvcInStorageClass("testPvc1", "default", &testStorageClass, annotations, nil, corev1.ClaimPending)
		pod := cc.CreateImporterTestPod(pvc, "testPvc1", nil)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
							Message:  `{"scratchSpaceRequired": true}`,
						},
					},
				},
			},
		}

		reconciler = createImportReconciler(pvc, pod)
		initPod := &corev1.Pod{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, initPod)
		Expect(err).ToNot(HaveOccurred())
		Expect(initPod).ToNot(BeNil())

		err = reconciler.updatePvcFromPod(pvc, pod, reconciler.log)
		Expect(err).ToNot(HaveOccurred())

		resPod := &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("\"importer-testPvc1\" not found"))
	})

	It("Should delete pod in favor of recreating with cache=trynone in case of OOMKilled", func() {
		annotations := map[string]string{
			cc.AnnEndpoint:             testEndPoint,
			cc.AnnSource:               cc.SourceRegistry,
			cc.AnnRegistryImportMethod: string(cdiv1.RegistryPullNode),
		}
		pvc := cc.CreatePvcInStorageClass("testPvc1", "default", &testStorageClass, annotations, nil, corev1.ClaimPending)
		reconciler = createImportReconciler(pvc)

		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		// First reconcile decides pods name, second creates it
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())

		// Simulate OOMKilled on pod
		resPod := &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).ToNot(HaveOccurred())
		resPod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: v1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 137,
							// This is an API
							// https://github.com/kubernetes/kubernetes/blob/e38531e9a2359c2ba1505cb04d62d6810edc616e/staging/src/k8s.io/cri-api/pkg/apis/runtime/v1/api.pb.go#L5822-L5823
							Reason: cc.OOMKilledReason,
						},
					},
				},
			},
		}
		err = reconciler.client.Status().Update(context.TODO(), resPod)
		Expect(err).ToNot(HaveOccurred())
		// Reconcile picks OOMKilled and deletes pod
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("\"importer-testPvc1\" not found"))
		// Next reconcile recreates pod with cache=trynone
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPod.Spec.Containers[0].Env).To(ContainElement(
			corev1.EnvVar{
				Name:  common.CacheMode,
				Value: common.CacheModeTryNone,
			},
		))
	})
})

var _ = Describe("Create Importer Pod", func() {
	var scratchPvcName = "scratchPvc"

	DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, scratchPvcName *string) {
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
		podArgs := &importerPodArgs{
			image:             testImage,
			verbose:           "5",
			pullPolicy:        testPullPolicy,
			podEnvVar:         podEnvVar,
			pvc:               pvc,
			scratchPvcName:    scratchPvcName,
			priorityClassName: pvc.Annotations[cc.AnnPriorityClassName],
		}
		pod, err := createImporterPod(context.TODO(), reconciler.log, reconciler.client, podArgs, map[string]string{})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying PVC owns pod")
		Expect(pod.GetOwnerReferences()).To(HaveLen(1))
		Expect(pod.GetOwnerReferences()[0].UID).To(Equal(pvc.GetUID()))
		By("Verifying volume mode is correct")
		if cc.GetVolumeMode(pvc) == corev1.PersistentVolumeBlock {
			Expect(pod.Spec.Containers[0].VolumeDevices[0].Name).To(Equal(cc.DataVolName))
			Expect(pod.Spec.Containers[0].VolumeDevices[0].DevicePath).To(Equal(common.WriteBlockPath))
			if scratchPvcName != nil {
				By("Verifying scratch space is set if available")
				Expect(pod.Spec.Containers[0].VolumeMounts).To(HaveLen(1))
				Expect(pod.Spec.Containers[0].VolumeMounts[0].Name).To(Equal(cc.ScratchVolName))
				Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal(common.ScratchDataDir))
			}
		} else {
			Expect(pod.Spec.Containers[0].VolumeMounts[0].Name).To(Equal(cc.DataVolName))
			Expect(pod.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal(common.ImporterDataDir))
			if scratchPvcName != nil {
				By("Verifying scratch space is set if available")
				Expect(pod.Spec.Containers[0].VolumeMounts).To(HaveLen(2))
				Expect(pod.Spec.Containers[0].VolumeMounts[1].Name).To(Equal(cc.ScratchVolName))
				Expect(pod.Spec.Containers[0].VolumeMounts[1].MountPath).To(Equal(common.ScratchDataDir))
			}
		}
		By("Verifying container spec is correct")
		Expect(pod.Spec.Containers[0].Image).To(Equal(testImage))
		Expect(pod.Spec.Containers[0].ImagePullPolicy).To(BeEquivalentTo(testPullPolicy))
		Expect(pod.Spec.Containers[0].Args[0]).To(Equal("-v=5"))
		Expect(pod.Spec.PriorityClassName).To(Equal(pvc.Annotations[cc.AnnPriorityClassName]))
	},
		Entry("should create pod with file system volume mode", cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnPodPhase: string(corev1.PodPending), cc.AnnImportPod: "podName", cc.AnnPriorityClassName: "p0"}, nil), nil),
		Entry("should create pod with block volume mode", createBlockPvc("testBlockPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnPodPhase: string(corev1.PodPending), cc.AnnImportPod: "podName", cc.AnnPriorityClassName: "p0"}, nil), nil),
		Entry("should create pod with file system volume mode and scratchspace", cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnPodPhase: string(corev1.PodPending), cc.AnnImportPod: "podName", cc.AnnPriorityClassName: "p0"}, nil), &scratchPvcName),
		Entry("should create pod with block volume mode and scratchspace", createBlockPvc("testBlockPvc1", "default", map[string]string{cc.AnnEndpoint: testEndPoint, cc.AnnPodPhase: string(corev1.PodPending), cc.AnnImportPod: "podName", cc.AnnPriorityClassName: "p0"}, nil), &scratchPvcName),
	)

	DescribeTable("should append current checkpoint name to importer pod", func(pvcName, checkpointID string) {
		pvc := cc.CreatePvc(pvcName, "default", map[string]string{cc.AnnCurrentCheckpoint: checkpointID, cc.AnnEndpoint: testEndPoint}, nil)
		pvc.Status.Phase = v1.ClaimBound

		suffix := fmt.Sprintf("%s-checkpoint-%s", pvcName, checkpointID)
		expectedName := fmt.Sprintf("importer-%s", suffix)
		if len(expectedName) > kvalidation.DNS1123SubdomainMaxLength {
			expectedName = naming.GetResourceName("importer", suffix)
		}

		reconciler := createImportReconciler(pvc)
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: pvcName, Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())

		// First reconcile sets cc.AnnImportPod
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: pvcName, Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.Annotations[cc.AnnImportPod]).To(Equal(expectedName))

		// Second reconcile creates pod
		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: pvcName, Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())

		resPod := &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "default"}, resPod)
		Expect(err).ToNot(HaveOccurred())
	},
		Entry("with short PVC and checkpoint names", "testPvc1", "snap1"),
		Entry("with long checkpoint name", "testPvc1", strings.Repeat("repeating-checkpoint-id-", 10)),
		Entry("with long PVC name", strings.Repeat("test-pvc-", 20), "snap1"),
		Entry("with long PVC and checkpoint names", strings.Repeat("test-pvc-", 20), strings.Repeat("repeating-checkpoint-id-", 10)),
	)

	It("should mount extra VDDK arguments ConfigMap when annotation is set", func() {
		pvcName := "testPvc1"
		podName := "testpod"
		extraArgs := "testing-123"
		annotations := map[string]string{
			cc.AnnEndpoint:         testEndPoint,
			cc.AnnImportPod:        podName,
			cc.AnnSource:           cc.SourceVDDK,
			cc.AnnVddkInitImageURL: "testing-vddk",
			cc.AnnVddkExtraArgs:    extraArgs,
		}
		pvc := cc.CreatePvcInStorageClass(pvcName, "default", &testStorageClass, annotations, nil, corev1.ClaimBound)
		reconciler := createImportReconciler(pvc)

		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: pvcName, Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())

		pod := &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: podName, Namespace: "default"}, pod)
		Expect(err).ToNot(HaveOccurred())

		found := false // Look for vddk-args mount
		for _, volume := range pod.Spec.Volumes {
			if volume.ConfigMap != nil && volume.ConfigMap.Name == extraArgs {
				found = true
			}
		}
		Expect(found).To(BeTrue())
	})
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
			source:             cc.SourceHTTP,
			contentType:        string(cdiv1.DataVolumeKubeVirt),
			imageSize:          "1G",
			certConfigMap:      "",
			diskID:             "",
			uuid:               "",
			readyFile:          "",
			doneFile:           "",
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
		pvcWithAnno := cc.CreatePvc("testPVCWithAnno", "default", map[string]string{cc.AnnSecret: "mysecret"}, nil)
		testSecret := createSecret("mysecret", "default", "mysecretkey", "mysecretstring", map[string]string{cc.AnnSecret: "mysecret"})
		reconciler := createImportReconciler(pvcWithAnno, testSecret)
		result := reconciler.getSecretName(pvcWithAnno)
		Expect(result).To(Equal("mysecret"))
	})

	It("should not find a secret", func() {
		pvcNoAnno := cc.CreatePvc("testPVCNoAnno", "default", nil, nil)
		testSecret := createSecret("mysecret2", "default", "mysecretkey2", "mysecretstring2", map[string]string{cc.AnnSecret: "mysecret2"})
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
		pvcWithAnno := cc.CreatePvc("testPVCWithAnno", "default", map[string]string{cc.AnnCertConfigMap: "configMapName"}, nil)
		reconciler := createImportReconciler(pvcWithAnno, testConfigMap)
		cdiConfig := &cdiv1.CDIConfig{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		cm, err := reconciler.getCertConfigMap(pvcWithAnno)
		Expect(err).ToNot(HaveOccurred())
		Expect(cm).To(Equal(testConfigMap.Name))
	})

	It("should return the configmap name if PVC has one defined, but doesn't exist", func() {
		pvcWithAnno := cc.CreatePvc("testPVCWithAnno", "default", map[string]string{cc.AnnCertConfigMap: "doesnotexist"}, nil)
		reconciler := createImportReconciler(pvcWithAnno)
		cdiConfig := &cdiv1.CDIConfig{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig)
		Expect(err).ToNot(HaveOccurred())
		cm, err := reconciler.getCertConfigMap(pvcWithAnno)
		Expect(err).ToNot(HaveOccurred())
		Expect(cm).To(Equal("doesnotexist"))
	})

	It("should return blank if the pvc has no annotation", func() {
		pvcNoAnno := cc.CreatePvc("testPVC", "default", nil, nil)
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

	DescribeTable("should", func(endpoint string, insecureHost string, isInsecure bool) {
		pvc := cc.CreatePvc("testPVC", "default", map[string]string{cc.AnnEndpoint: endpoint}, nil)
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
		Entry("return true on endpoint with no port, and host defined", endpointNoPort, host, true),
		Entry("return true on endpoint with port, and host with port", endpointWithPort, hostWithPort, true),
		Entry("return false on endpoint with no port, and host with port", endpointNoPort, hostWithPort, false),
		Entry("return false on endpoint with port, and host defined", endpointWithPort, host, false),
		Entry("return false on endpoint with no port, and blank host", endpointNoPort, "", false),
		Entry("return false on blank endpoint, and host defined", "", host, false),
	)
})

var _ = Describe("GetContentType", func() {
	pvcNoAnno := cc.CreatePvc("testPVCNoAnno", "default", nil, nil)
	pvcArchiveAnno := cc.CreatePvc("testPVCArchiveAnno", "default", map[string]string{cc.AnnContentType: string(cdiv1.DataVolumeArchive)}, nil)
	pvcKubevirtAnno := cc.CreatePvc("testPVCKubevirtAnno", "default", map[string]string{cc.AnnContentType: string(cdiv1.DataVolumeKubeVirt)}, nil)
	pvcInvalidValue := cc.CreatePvc("testPVCInvalidValue", "default", map[string]string{cc.AnnContentType: "iaminvalid"}, nil)

	DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, expectedResult cdiv1.DataVolumeContentType) {
		result := cc.GetPVCContentType(pvc)
		Expect(result).To(Equal(expectedResult))
	},
		Entry("return kubevirt contenttype if no annotation provided", pvcNoAnno, cdiv1.DataVolumeKubeVirt),
		Entry("return archive contenttype if archive annotation present", pvcArchiveAnno, cdiv1.DataVolumeArchive),
		Entry("return kubevirt contenttype if kubevirt annotation present", pvcKubevirtAnno, cdiv1.DataVolumeKubeVirt),
		Entry("return kubevirt contenttype if invalid annotation provided", pvcInvalidValue, cdiv1.DataVolumeKubeVirt),
	)
})

var _ = Describe("getSource", func() {
	pvcNoAnno := cc.CreatePvc("testPVCNoAnno", "default", nil, nil)
	pvcNoneAnno := cc.CreatePvc("testPVCNoneAnno", "default", map[string]string{cc.AnnSource: cc.SourceNone}, nil)
	pvcGlanceAnno := cc.CreatePvc("testPVCNoneAnno", "default", map[string]string{cc.AnnSource: cc.SourceGlance}, nil)
	pvcInvalidValue := cc.CreatePvc("testPVCInvalidValue", "default", map[string]string{cc.AnnSource: "iaminvalid"}, nil)
	pvcRegistryAnno := cc.CreatePvc("testPVCRegistryAnno", "default", map[string]string{cc.AnnSource: cc.SourceRegistry}, nil)
	pvcImageIOAnno := cc.CreatePvc("testPVCImageIOAnno", "default", map[string]string{cc.AnnSource: cc.SourceImageio}, nil)
	pvcVDDKAnno := cc.CreatePvc("testPVCVDDKAnno", "default", map[string]string{cc.AnnSource: cc.SourceVDDK}, nil)

	DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, expectedResult string) {
		result := cc.GetSource(pvc)
		Expect(result).To(BeEquivalentTo(expectedResult))
	},
		Entry("return none if none annotation provided", pvcNoneAnno, cc.SourceNone),
		Entry("return http if no annotation provided", pvcNoAnno, cc.SourceHTTP),
		Entry("return glance if glance annotation provided", pvcGlanceAnno, cc.SourceGlance),
		Entry("return http if invalid annotation provided", pvcInvalidValue, cc.SourceHTTP),
		Entry("return registry if registry annotation provided", pvcRegistryAnno, cc.SourceRegistry),
		Entry("return imageio if imageio annotation provided", pvcImageIOAnno, cc.SourceImageio),
		Entry("return vddk if vddk annotation provided", pvcVDDKAnno, cc.SourceVDDK),
	)
})

var _ = Describe("GetEndpoint", func() {
	pvcNoAnno := cc.CreatePvc("testPVCNoAnno", "default", nil, nil)
	pvcWithAnno := cc.CreatePvc("testPVCWithAnno", "default", map[string]string{cc.AnnEndpoint: "http://test"}, nil)
	pvcNoValue := cc.CreatePvc("testPVCNoValue", "default", map[string]string{cc.AnnEndpoint: ""}, nil)

	DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, expectedResult string, expectErr bool) {
		result, err := cc.GetEndpoint(pvc)
		Expect(result).To(BeEquivalentTo(expectedResult))
		if expectErr {
			Expect(err).To(HaveOccurred())
		} else {
			Expect(err).ToNot(HaveOccurred())
		}
	},
		Entry("return blank and error if no annotation provided", pvcNoAnno, "", true),
		Entry("return value and no error if valid annotation provided", pvcWithAnno, "http://test", false),
		Entry("return blank and error if blank annotation provided", pvcNoValue, "", true),
	)
})

func createImportReconciler(objects ...runtime.Object) *ImportReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register cdi types with the runtime scheme.
	s := scheme.Scheme
	_ = cdiv1.AddToScheme(s)

	objs = append(objs, cc.MakeEmptyCDICR())

	cdiConfig := cc.MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	objs = append(objs, cdiConfig)

	// Create a fake client to mock API calls.
	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()

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
		installerLabels: map[string]string{
			common.AppKubernetesPartOfLabel:  "testing",
			common.AppKubernetesVersionLabel: "v0.0.0-tests",
		},
	}
	return r
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
			Value: uid,
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
			Name:  common.ImporterPullMethod,
			Value: podEnvVar.pullMethod,
		},
		{
			Name:  common.ImporterReadyFile,
			Value: podEnvVar.readyFile,
		},
		{
			Name:  common.ImporterDoneFile,
			Value: podEnvVar.doneFile,
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
		{
			Name:  common.CacheMode,
			Value: podEnvVar.cacheMode,
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

type FakeFeatureGates struct {
	honorWaitForFirstConsumerEnabled bool
	claimAdoptionEnabled             bool
	webhookPvcRenderingEnabled       bool
}

func (f *FakeFeatureGates) HonorWaitForFirstConsumerEnabled() (bool, error) {
	return f.honorWaitForFirstConsumerEnabled, nil
}

func (f *FakeFeatureGates) ClaimAdoptionEnabled() (bool, error) {
	return f.claimAdoptionEnabled, nil
}

func (f *FakeFeatureGates) WebhookPvcRenderingEnabled() (bool, error) {
	return f.webhookPvcRenderingEnabled, nil
}

func createPendingPvc(name, ns string, annotations, labels map[string]string) *v1.PersistentVolumeClaim {
	return cc.CreatePvcInStorageClass(name, ns, nil, annotations, labels, v1.ClaimPending)
}

func createSecret(name, ns, accessKey, secretKey string, labels map[string]string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Type: "Opaque",
		Data: map[string][]byte{
			bootstrapapi.BootstrapTokenIDKey:           []byte(accessKey),
			bootstrapapi.BootstrapTokenSecretKey:       []byte(secretKey),
			bootstrapapi.BootstrapTokenUsageSigningKey: []byte("true"),
		},
	}
}

func updateCdiWithTestNodePlacement(c client.Client) sdkapi.NodePlacement {
	cr := &cdiv1.CDI{}
	err := c.Get(context.TODO(), types.NamespacedName{Name: "cdi"}, cr)
	Expect(err).ToNot(HaveOccurred())

	workloads := sdkapi.NodePlacement{
		NodeSelector: map[string]string{"kubernetes.io/arch": "amd64"},
		Tolerations:  []v1.Toleration{{Key: "test", Value: "123"}},
		Affinity: &v1.Affinity{
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
		},
	}

	cr.Spec.Workloads = workloads
	err = c.Update(context.TODO(), cr)
	Expect(err).ToNot(HaveOccurred())

	placement, err := cc.GetWorkloadNodePlacement(context.TODO(), c)
	Expect(err).ToNot(HaveOccurred())
	Expect(*placement).To(Equal(workloads))

	return workloads
}
