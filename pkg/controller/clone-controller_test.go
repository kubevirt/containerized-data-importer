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
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/triple"
)

var (
	apiServerKey     *rsa.PrivateKey
	apiServerKeyOnce sync.Once
	cloneLog         = logf.Log.WithName("clone-controller-test")
)

type fakeCertGenerator struct {
}

func (cg *fakeCertGenerator) MakeClientCert(name string, groups []string, duration time.Duration) ([]byte, []byte, error) {
	return []byte("foo"), []byte("bar"), nil
}

func (cg *fakeCertGenerator) MakeServerCert(namespace, service string, duration time.Duration) ([]byte, []byte, error) {
	return []byte("foo"), []byte("bar"), nil
}

type FakeValidator struct {
	match     string
	Operation token.Operation
	Name      string
	Namespace string
	Resource  metav1.GroupVersionResource
	Params    map[string]string
}

var _ = Describe("Clone controller reconcile loop", func() {
	var (
		reconciler *CloneReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			reconciler = nil
		}
	})

	It("Should return success if a PVC with no annotations is passed, due to it being ignored", func() {
		reconciler = createCloneReconciler(createPvc("testPvc1", "default", map[string]string{}, nil))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should return success if no PVC can be found, due to it not existing", func() {
		reconciler = createCloneReconciler()
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should return success if no PVC can be found due to not existing in passed namespace", func() {
		reconciler = createCloneReconciler(createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint}, nil))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "invalid"}})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should return success if a PVC with clone request annotation and cloneof is passed, due to it being ignored", func() {
		reconciler = createCloneReconciler(createPvc("testPvc1", "default", map[string]string{AnnCloneRequest: "cloneme", AnnCloneOf: "something"}, nil))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should return success if target pod is not ready", func() {
		reconciler = createCloneReconciler(createPvc("testPvc1", "default", map[string]string{AnnCloneRequest: "cloneme"}, nil))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should return error if target pod annotation is invalid", func() {
		reconciler = createCloneReconciler(createPvc("testPvc1", "default", map[string]string{AnnCloneRequest: "cloneme", AnnPodReady: "invalid"}, nil))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("error parsing %s annotation", AnnPodReady)))
	})

	It("Should create source pod name and add finalizer", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{
			AnnCloneRequest:     "default/source",
			AnnPodReady:         "true",
			AnnCloneToken:       "foobaz",
			AnnUploadClientName: "uploadclient"}, nil)
		reconciler = createCloneReconciler(testPvc, createPvc("source", "default", map[string]string{}, nil))
		By("Setting up the match token")
		reconciler.tokenValidator.(*FakeValidator).match = "foobaz"
		reconciler.tokenValidator.(*FakeValidator).Name = "source"
		reconciler.tokenValidator.(*FakeValidator).Namespace = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetNamespace"] = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetName"] = "testPvc1"
		By("Verifying no source pod exists")
		sourcePod, err := reconciler.findCloneSourcePod(testPvc)
		Expect(sourcePod).To(BeNil())
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying no source pod exists")
		sourcePod, err = reconciler.findCloneSourcePod(testPvc)
		Expect(sourcePod).To(BeNil())
		By("Verifying the PVC now has a source pod name")
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(testPvc.Annotations[AnnCloneSourcePod]).To(Equal("default-testPvc1-source-pod"))
		Expect(reconciler.hasFinalizer(testPvc, cloneSourcePodFinalizer)).To(BeTrue())
	})

	DescribeTable("Should NOT create new source pod if source PVC is in use", func(podFunc func(*corev1.PersistentVolumeClaim) *corev1.Pod) {
		testPvc := createPvc("testPvc1", "default", map[string]string{
			AnnCloneRequest:     "default/source",
			AnnPodReady:         "true",
			AnnCloneToken:       "foobaz",
			AnnUploadClientName: "uploadclient",
			AnnCloneSourcePod:   "default-testPvc1-source-pod"}, nil)
		sourcePvc := createPvc("source", "default", map[string]string{}, nil)
		reconciler = createCloneReconciler(testPvc, sourcePvc, podFunc(sourcePvc))
		By("Setting up the match token")
		reconciler.tokenValidator.(*FakeValidator).match = "foobaz"
		reconciler.tokenValidator.(*FakeValidator).Name = "source"
		reconciler.tokenValidator.(*FakeValidator).Namespace = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetNamespace"] = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetName"] = "testPvc1"
		By("Verifying no source pod exists")
		sourcePod, err := reconciler.findCloneSourcePod(testPvc)
		Expect(sourcePod).To(BeNil())
		result, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Requeue).To(BeTrue())
		By("Verifying source pod does not exist")
		sourcePod, err = reconciler.findCloneSourcePod(testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(sourcePod).To(BeNil())
		By("Checking events recorded")
		close(reconciler.recorder.(*record.FakeRecorder).Events)
		found := false
		for event := range reconciler.recorder.(*record.FakeRecorder).Events {
			if strings.Contains(event, "CloneSourceInUse") {
				found = true
			}
		}
		Expect(found).To(BeTrue())
		reconciler = nil
	},
		Entry("read/write", func(pvc *corev1.PersistentVolumeClaim) *corev1.Pod {
			return podUsingPVC(pvc, false)
		}),
	)

	DescribeTable("Should create new source pod if none exists, and target pod is marked ready and", func(podFunc func(*corev1.PersistentVolumeClaim) *corev1.Pod) {
		testPvc := createPvc("testPvc1", "default", map[string]string{
			AnnCloneRequest:     "default/source",
			AnnPodReady:         "true",
			AnnCloneToken:       "foobaz",
			AnnUploadClientName: "uploadclient",
			AnnCloneSourcePod:   "default-testPvc1-source-pod",
			AnnPodNetwork:       "net1"}, nil)
		sourcePvc := createPvc("source", "default", map[string]string{}, nil)
		otherSourcePod := podFunc(sourcePvc)
		objs := []runtime.Object{testPvc, sourcePvc}
		if otherSourcePod != nil {
			objs = append(objs, otherSourcePod)
		}
		reconciler = createCloneReconciler(objs...)
		By("Setting up the match token")
		reconciler.tokenValidator.(*FakeValidator).match = "foobaz"
		reconciler.tokenValidator.(*FakeValidator).Name = "source"
		reconciler.tokenValidator.(*FakeValidator).Namespace = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetNamespace"] = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetName"] = "testPvc1"
		By("Verifying no source pod exists")
		sourcePod, err := reconciler.findCloneSourcePod(testPvc)
		Expect(sourcePod).To(BeNil())
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying source pod exists")
		sourcePod, err = reconciler.findCloneSourcePod(testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(sourcePod).ToNot(BeNil())
		Expect(sourcePod.GetLabels()[CloneUniqueID]).To(Equal("default-testPvc1-source-pod"))
		By("Verifying source pod annotations passed from pvc")
		Expect(sourcePod.GetAnnotations()[AnnPodNetwork]).To(Equal("net1"))
		Expect(sourcePod.Spec.Affinity).ToNot(BeNil())
		Expect(sourcePod.Spec.Affinity.PodAffinity).ToNot(BeNil())
		l := len(sourcePod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution)
		Expect(l > 0).To(BeTrue())
		pa := sourcePod.Spec.Affinity.PodAffinity.PreferredDuringSchedulingIgnoredDuringExecution[l-1]
		epa := corev1.WeightedPodAffinityTerm{
			Weight: 100,
			PodAffinityTerm: corev1.PodAffinityTerm{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      common.UploadTargetLabel,
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{string(testPvc.UID)},
						},
					},
				},
				Namespaces:  []string{"default"},
				TopologyKey: corev1.LabelHostname,
			},
		}
		Expect(pa).To(Equal(epa))
	},
		Entry("no pods are using source PVC", func(pvc *corev1.PersistentVolumeClaim) *corev1.Pod {
			return nil
		}),
		Entry("readonly pod using source PVC", func(pvc *corev1.PersistentVolumeClaim) *corev1.Pod {
			return podUsingPVC(pvc, true)
		}),
		Entry("other clone source pod using source PVC", func(pvc *corev1.PersistentVolumeClaim) *corev1.Pod {
			pod := podUsingPVC(pvc, true)
			pod.Labels = map[string]string{"cdi.kubevirt.io": "cdi-clone-source"}
			return pod
		}),
	)

	It("Should error with missing upload client name annotation if none provided", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{
			AnnCloneRequest: "default/source", AnnPodReady: "true", AnnCloneToken: "foobaz", AnnCloneSourcePod: "default-testPvc1-source-pod"}, nil)
		reconciler = createCloneReconciler(testPvc, createPvc("source", "default", map[string]string{}, nil))
		By("Setting up the match token")
		reconciler.tokenValidator.(*FakeValidator).match = "foobaz"
		reconciler.tokenValidator.(*FakeValidator).Name = "source"
		reconciler.tokenValidator.(*FakeValidator).Namespace = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetNamespace"] = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetName"] = "testPvc1"
		By("Verifying no source pod exists")
		sourcePod, err := reconciler.findCloneSourcePod(testPvc)
		Expect(sourcePod).To(BeNil())
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("missing required " + AnnUploadClientName + " annotation"))
	})

	It("Should update the PVC from the pod status", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{
			AnnCloneRequest: "default/source", AnnPodReady: "true", AnnCloneToken: "foobaz", AnnUploadClientName: "uploadclient", AnnCloneSourcePod: "default-testPvc1-source-pod"}, nil)
		reconciler = createCloneReconciler(testPvc, createPvc("source", "default", map[string]string{}, nil))
		By("Setting up the match token")
		reconciler.tokenValidator.(*FakeValidator).match = "foobaz"
		reconciler.tokenValidator.(*FakeValidator).Name = "source"
		reconciler.tokenValidator.(*FakeValidator).Namespace = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetNamespace"] = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetName"] = "testPvc1"
		By("Verifying no source pod exists")
		sourcePod, err := reconciler.findCloneSourcePod(testPvc)
		Expect(sourcePod).To(BeNil())
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying source pod exists")
		sourcePod, err = reconciler.findCloneSourcePod(testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(sourcePod.GetLabels()[CloneUniqueID]).To(Equal("default-testPvc1-source-pod"))
	})

	It("Should update the cloneof when complete", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{
			AnnCloneRequest: "default/source", AnnPodReady: "true", AnnCloneToken: "foobaz", AnnUploadClientName: "uploadclient"}, nil)
		reconciler = createCloneReconciler(testPvc, createPvc("source", "default", map[string]string{}, nil))
		By("Setting up the match token")
		reconciler.tokenValidator.(*FakeValidator).match = "foobaz"
		reconciler.tokenValidator.(*FakeValidator).Name = "source"
		reconciler.tokenValidator.(*FakeValidator).Namespace = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetNamespace"] = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetName"] = "testPvc1"
		By("Verifying no source pod exists")
		sourcePod, err := reconciler.findCloneSourcePod(testPvc)
		Expect(sourcePod).To(BeNil())
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying the PVC now has a source pod name")
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(testPvc.Annotations[AnnCloneSourcePod]).To(Equal("default-testPvc1-source-pod"))
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying source pod exists")
		sourcePod, err = reconciler.findCloneSourcePod(testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(sourcePod.GetLabels()[CloneUniqueID]).To(Equal("default-testPvc1-source-pod"))
		By("Verifying the PVC now has a finalizer")
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciler.hasFinalizer(testPvc, cloneSourcePodFinalizer)).To(BeTrue())
		By("Updating the PVC to completed")
		testPvc.Annotations = map[string]string{
			AnnCloneRequest: "default/source", AnnPodReady: "true", AnnCloneToken: "foobaz", AnnUploadClientName: "uploadclient", AnnCloneSourcePod: "default-testPvc1-source-pod", AnnPodPhase: string(corev1.PodSucceeded)}
		err = reconciler.client.Update(context.TODO(), testPvc)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(testPvc.GetAnnotations()[AnnCloneOf]).To(Equal("true"))
		sourcePod, err = reconciler.findCloneSourcePod(testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(sourcePod).ToNot(BeNil())
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Checking error event recorded")
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring(cloneComplete))
		sourcePod, err = reconciler.findCloneSourcePod(testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(sourcePod).To(BeNil())
		By("Verifying the PVC does not have a finalizer")
		testPvc = &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciler.hasFinalizer(testPvc, cloneSourcePodFinalizer)).To(BeFalse())
	})

	It("Should update the cloneof when complete, block mode", func() {
		testPvc := createBlockPvc("testPvc1", "default", map[string]string{
			AnnCloneRequest: "default/source", AnnPodReady: "true", AnnCloneToken: "foobaz", AnnUploadClientName: "uploadclient"}, nil)
		reconciler = createCloneReconciler(testPvc, createBlockPvc("source", "default", map[string]string{}, nil))
		By("Setting up the match token")
		reconciler.tokenValidator.(*FakeValidator).match = "foobaz"
		reconciler.tokenValidator.(*FakeValidator).Name = "source"
		reconciler.tokenValidator.(*FakeValidator).Namespace = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetNamespace"] = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetName"] = "testPvc1"
		By("Verifying no source pod exists")
		sourcePod, err := reconciler.findCloneSourcePod(testPvc)
		Expect(sourcePod).To(BeNil())
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying the PVC now has a source pod name")
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(testPvc.Annotations[AnnCloneSourcePod]).To(Equal("default-testPvc1-source-pod"))
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying source pod exists")
		sourcePod, err = reconciler.findCloneSourcePod(testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(sourcePod.GetLabels()[CloneUniqueID]).To(Equal("default-testPvc1-source-pod"))
		By("Verifying the PVC now has a finalizer")
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(reconciler.hasFinalizer(testPvc, cloneSourcePodFinalizer)).To(BeTrue())
		By("Updating the PVC to completed")
		testPvc.Annotations = map[string]string{
			AnnCloneRequest: "default/source", AnnPodReady: "true", AnnCloneToken: "foobaz", AnnUploadClientName: "uploadclient", AnnCloneSourcePod: "default-testPvc1-source-pod", AnnPodPhase: string(corev1.PodSucceeded)}
		err = reconciler.client.Update(context.TODO(), testPvc)
		Expect(err).ToNot(HaveOccurred())
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(testPvc.GetAnnotations()[AnnCloneOf]).To(Equal("true"))
		sourcePod, err = reconciler.findCloneSourcePod(testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(sourcePod).ToNot(BeNil())
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Checking error event recorded")
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring(cloneComplete))
		sourcePod, err = reconciler.findCloneSourcePod(testPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(sourcePod).To(BeNil())
	})

	It("Should error when source and target volume modes do not match (fs->block)", func() {
		testPvc := createBlockPvc("testPvc1", "default", map[string]string{
			AnnCloneRequest: "default/source", AnnPodReady: "true", AnnCloneToken: "foobaz", AnnUploadClientName: "uploadclient", AnnCloneSourcePod: "default-testPvc1-source-pod"}, nil)
		reconciler = createCloneReconciler(testPvc, createPvc("source", "default", map[string]string{}, nil))
		By("Setting up the match token")
		reconciler.tokenValidator.(*FakeValidator).match = "foobaz"
		reconciler.tokenValidator.(*FakeValidator).Name = "source"
		reconciler.tokenValidator.(*FakeValidator).Namespace = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetNamespace"] = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetName"] = "testPvc1"
		By("Verifying no source pod exists")
		sourcePod, err := reconciler.findCloneSourcePod(testPvc)
		Expect(sourcePod).To(BeNil())
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("source volumeMode (Filesystem) and target volumeMode (Block) do not match"))
	})

	It("Should error when source and target volume modes do not match (block->fs)", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{
			AnnCloneRequest: "default/source", AnnPodReady: "true", AnnCloneToken: "foobaz", AnnUploadClientName: "uploadclient", AnnCloneSourcePod: "default-testPvc1-source-pod"}, nil)
		reconciler = createCloneReconciler(testPvc, createBlockPvc("source", "default", map[string]string{}, nil))
		By("Setting up the match token")
		reconciler.tokenValidator.(*FakeValidator).match = "foobaz"
		reconciler.tokenValidator.(*FakeValidator).Name = "source"
		reconciler.tokenValidator.(*FakeValidator).Namespace = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetNamespace"] = "default"
		reconciler.tokenValidator.(*FakeValidator).Params["targetName"] = "testPvc1"
		By("Verifying no source pod exists")
		sourcePod, err := reconciler.findCloneSourcePod(testPvc)
		Expect(sourcePod).To(BeNil())
		_, err = reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("source volumeMode (Block) and target volumeMode (Filesystem) do not match"))
	})

})

var _ = Describe("ParseCloneRequestAnnotation", func() {
	It("should return false/empty/empty if no annotation exists", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{}, nil)
		exists, ns, name := ParseCloneRequestAnnotation(pvc)
		Expect(exists).To(BeFalse())
		Expect(ns).To(BeEmpty())
		Expect(name).To(BeEmpty())
	})

	It("should return false/empty/empty if annotation is invalid", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{AnnCloneRequest: "default"}, nil)
		exists, ns, name := ParseCloneRequestAnnotation(pvc)
		Expect(exists).To(BeFalse())
		Expect(ns).To(BeEmpty())
		Expect(name).To(BeEmpty())
		pvc = createPvc("testPvc1", "default", map[string]string{AnnCloneRequest: "default/test/something"}, nil)
		exists, ns, name = ParseCloneRequestAnnotation(pvc)
		Expect(exists).To(BeFalse())
		Expect(ns).To(BeEmpty())
		Expect(name).To(BeEmpty())
	})

	It("should return true/default/test if annotation is valid", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{AnnCloneRequest: "default/test"}, nil)
		exists, ns, name := ParseCloneRequestAnnotation(pvc)
		Expect(exists).To(BeTrue())
		Expect(ns).To(Equal("default"))
		Expect(name).To(Equal("test"))
	})
})

var _ = Describe("CloneSourcePodName", func() {
	It("Should be unique and deterministic", func() {
		pvc1d := createPvc("testPvc1", "default", map[string]string{AnnCloneRequest: "default/test"}, nil)
		pvc1d2 := createPvc("testPvc1", "default2", map[string]string{AnnCloneRequest: "default/test"}, nil)
		pvc2d1 := createPvc("testPvc2", "default", map[string]string{AnnCloneRequest: "default/test"}, nil)
		pvcSimilar := createPvc("testP", "vc1default", map[string]string{AnnCloneRequest: "default/test"}, nil)
		podName1d := createCloneSourcePodName(pvc1d)
		podName1dagain := createCloneSourcePodName(pvc1d)
		By("Verifying rerunning getloneSourcePodName on same PVC I get same name")
		Expect(podName1d).To(Equal(podName1dagain))
		By("Verifying different namespace but same name I get different pod name")
		podName1d2 := createCloneSourcePodName(pvc1d2)
		Expect(podName1d).NotTo(Equal(podName1d2))
		By("Verifying same namespace but different name I get different pod name")
		podName2d1 := createCloneSourcePodName(pvc2d1)
		Expect(podName1d).NotTo(Equal(podName2d1))
		By("Verifying concatenated ns/name of same characters I get different pod name")
		podNameSimilar := createCloneSourcePodName(pvcSimilar)
		Expect(podName1d).NotTo(Equal(podNameSimilar))
	})
})

var _ = Describe("Update PVC", func() {
	var (
		reconciler *CloneReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			close(reconciler.recorder.(*record.FakeRecorder).Events)
			reconciler = nil
		}
	})
	It("Should update AnnPodRestarts on pvc from source pod restarts", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{AnnCloneRequest: "default/test"}, nil)
		pod := createSourcePod(testPvc, string(testPvc.GetUID()))
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodFailed,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					RestartCount: 2,
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Message:  "I went poof",
						},
					},
				},
			},
		}
		reconciler = createCloneReconciler(testPvc, createPvc("source", "default", map[string]string{}, nil))

		err := reconciler.updatePvcFromPod(pod, testPvc, reconciler.log)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying the pvc has original restart count")
		actualPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, actualPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualPvc.Annotations[AnnPodRestarts]).To(Equal("2"))
	})

	It("Should not update AnnPodRestarts on pvc from source pod if pod has lower restart count value", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{
			AnnCloneRequest: "default/test",
			AnnPodRestarts:  "3"},
			nil)
		pod := createSourcePod(testPvc, string(testPvc.GetUID()))
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodFailed,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					RestartCount: 2,
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Message:  "I went poof",
						},
					},
				},
			},
		}
		reconciler = createCloneReconciler(testPvc, createPvc("source", "default", map[string]string{}, nil))

		err := reconciler.updatePvcFromPod(pod, testPvc, reconciler.log)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying the pvc has original restart count")
		actualPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, actualPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualPvc.Annotations[AnnPodRestarts]).To(Equal("3"))
	})
})

var _ = Describe("TokenValidation", func() {
	g := token.NewGenerator(common.CloneTokenIssuer, getAPIServerKey(), 5*time.Minute)
	v := newCloneTokenValidator(&getAPIServerKey().PublicKey)

	goodTokenData := func() *token.Payload {
		return &token.Payload{
			Operation: token.OperationClone,
			Name:      "source",
			Namespace: "sourcens",
			Resource: metav1.GroupVersionResource{
				Resource: "persistentvolumeclaims",
			},
			Params: map[string]string{
				"targetName":      "target",
				"targetNamespace": "targetns",
			},
		}
	}

	source := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source",
			Namespace: "sourcens",
		},
	}

	badOperation := goodTokenData()
	badOperation.Operation = token.OperationUpload

	badSourceName := goodTokenData()
	badSourceName.Name = "foo"

	badSourceNamespace := goodTokenData()
	badSourceNamespace.Namespace = "foo"

	badResource := goodTokenData()
	badResource.Resource.Resource = "foo"

	badTargetName := goodTokenData()
	badTargetName.Params["targetName"] = "foo"

	badTargetNamespace := goodTokenData()
	badTargetNamespace.Params["targetNamespace"] = "foo"

	missingParams := goodTokenData()
	missingParams.Params = nil

	DescribeTable("should", func(p *token.Payload, expectedSuccess bool) {
		tokenString, err := g.Generate(p)
		if err != nil {
			panic("error generating token")
		}

		target := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "target",
				Namespace: "targetns",
				Annotations: map[string]string{
					AnnCloneToken: tokenString,
				},
			},
		}
		err = validateCloneToken(v, source, target)
		if expectedSuccess {
			Expect(err).ToNot(HaveOccurred())
			Expect(reflect.DeepEqual(p, goodTokenData())).To(BeTrue())
		} else {
			Expect(err).To(HaveOccurred())
			Expect(reflect.DeepEqual(p, goodTokenData())).To(BeFalse())
		}
	},
		Entry("succeed", goodTokenData(), true),
		Entry("fail on bad operation", badOperation, false),
		Entry("fail on bad sourceName", badSourceName, false),
		Entry("fail on bad sourceNamespace", badSourceNamespace, false),
		Entry("fail on bad resource", badResource, false),
		Entry("fail on bad targetName", badTargetName, false),
		Entry("fail on bad targetNamespace", badTargetNamespace, false),
		Entry("fail on bad missing parameters", missingParams, false),
	)
})

func createCloneReconciler(objects ...runtime.Object) *CloneReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, MakeEmptyCDICR())
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		DefaultPodResourceRequirements: createDefaultPodResourceRequirements(int64(0), int64(0), int64(0), int64(0)),
	}
	objs = append(objs, cdiConfig)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	rec := record.NewFakeRecorder(1)
	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	// Create a ReconcileMemcached object with the scheme and fake client.
	return &CloneReconciler{
		client:   cl,
		scheme:   s,
		log:      cloneLog,
		recorder: rec,
		tokenValidator: &FakeValidator{
			Params: make(map[string]string, 0),
		},
		image:               testImage,
		clientCertGenerator: &fakeCertGenerator{},
		serverCAFetcher:     &fetcher.MemCertBundleFetcher{Bundle: []byte("baz")},
	}
}

func testCreateClientKeyAndCert(ca *triple.KeyPair, commonName string, organizations []string) ([]byte, []byte, error) {
	return []byte("foo"), []byte("bar"), nil
}

func getAPIServerKey() *rsa.PrivateKey {
	apiServerKeyOnce.Do(func() {
		apiServerKey, _ = rsa.GenerateKey(rand.Reader, 2048)
	})
	return apiServerKey
}

func (v *FakeValidator) Validate(value string) (*token.Payload, error) {
	if value != v.match {
		return nil, fmt.Errorf("Token does not match expected")
	}
	resource := metav1.GroupVersionResource{
		Resource: "persistentvolumeclaims",
	}
	return &token.Payload{
		Name:      v.Name,
		Namespace: v.Namespace,
		Operation: token.OperationClone,
		Resource:  resource,
		Params:    v.Params,
	}, nil
}

func createClonePvc(sourceNamespace, sourceName, targetNamespace, targetName string, annotations, labels map[string]string) *corev1.PersistentVolumeClaim {
	return createClonePvcWithSize(sourceNamespace, sourceName, targetNamespace, targetName, annotations, labels, "1G")
}

func createClonePvcWithSize(sourceNamespace, sourceName, targetNamespace, targetName string, annotations, labels map[string]string, size string) *corev1.PersistentVolumeClaim {
	tokenData := &token.Payload{
		Operation: token.OperationClone,
		Name:      sourceName,
		Namespace: sourceNamespace,
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "persistentvolumeclaims",
		},
		Params: map[string]string{
			"targetNamespace": targetNamespace,
			"targetName":      targetName,
		},
	}

	g := token.NewGenerator(common.CloneTokenIssuer, getAPIServerKey(), 5*time.Minute)

	tokenString, err := g.Generate(tokenData)
	if err != nil {
		panic("error generating token")
	}

	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[AnnCloneRequest] = fmt.Sprintf("%s/%s", sourceNamespace, sourceName)
	annotations[AnnCloneToken] = tokenString
	annotations[AnnUploadClientName] = "FOOBAR"

	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        targetName,
			Namespace:   targetNamespace,
			Annotations: annotations,
			Labels:      labels,
			UID:         "pvc-uid",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadOnlyMany, corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(size),
				},
			},
		},
	}
}

func createCloneBlockPvc(sourceNamespace, sourceName, targetNamespace, targetName string, annotations, labels map[string]string) *corev1.PersistentVolumeClaim {
	pvc := createClonePvc(sourceNamespace, sourceName, targetNamespace, targetName, annotations, labels)
	VolumeMode := corev1.PersistentVolumeBlock
	pvc.Spec.VolumeMode = &VolumeMode
	return pvc
}

func createSourcePod(pvc *corev1.PersistentVolumeClaim, pvcUID string) *corev1.Pod {
	_, _, sourcePvcName := ParseCloneRequestAnnotation(pvc)
	podName := fmt.Sprintf("%s-%s-", common.ClonerSourcePodName, sourcePvcName)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: podName,
			Annotations: map[string]string{
				AnnCreatedBy: "yes",
				AnnOwnerRef:  fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name),
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue, //filtered by the podInformer
				common.CDIComponentLabel: common.ClonerSourcePodName,
				// this label is used when searching for a pvc's cloner source pod.
				CloneUniqueID:          pvcUID + "-source-pod",
				common.PrometheusLabel: "",
			},
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser: &[]int64{0}[0],
			},
			Containers: []corev1.Container{
				{
					Name:            common.ClonerSourcePodName,
					Image:           "test/mycloneimage",
					ImagePullPolicy: corev1.PullAlways,
					Env: []corev1.EnvVar{
						{
							Name:  "CLIENT_KEY",
							Value: "bar",
						},
						{
							Name:  "CLIENT_CERT",
							Value: "foo",
						},
						{
							Name:  "SERVER_CA_CERT",
							Value: string("baz"),
						},
						{
							Name:  "UPLOAD_URL",
							Value: GetUploadServerURL(pvc.Namespace, pvc.Name, common.UploadPathSync),
						},
						{
							Name:  common.OwnerUID,
							Value: "",
						},
					},
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
			Volumes: []corev1.Volume{
				{
					Name: DataVolName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: sourcePvcName,
							ReadOnly:  false,
						},
					},
				},
			},
		},
	}

	var volumeMode corev1.PersistentVolumeMode
	var addVars []corev1.EnvVar

	if pvc.Spec.VolumeMode != nil {
		volumeMode = *pvc.Spec.VolumeMode
	} else {
		volumeMode = corev1.PersistentVolumeFilesystem
	}

	if volumeMode == corev1.PersistentVolumeBlock {
		pod.Spec.Containers[0].VolumeDevices = addVolumeDevices()
		addVars = []corev1.EnvVar{
			{
				Name:  "VOLUME_MODE",
				Value: "block",
			},
			{
				Name:  "MOUNT_POINT",
				Value: common.WriteBlockPath,
			},
		}
		pod.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsUser: &[]int64{0}[0],
		}
	} else {
		pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      DataVolName,
				MountPath: common.ClonerMountPath,
			},
		}
		addVars = []corev1.EnvVar{
			{
				Name:  "VOLUME_MODE",
				Value: "filesystem",
			},
			{
				Name:  "MOUNT_POINT",
				Value: common.ClonerMountPath,
			},
		}
	}

	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, addVars...)

	return pod
}
