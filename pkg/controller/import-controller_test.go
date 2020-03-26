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

	"k8s.io/apimachinery/pkg/runtime"
	cdifake "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned/fake"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sfake "k8s.io/client-go/kubernetes/fake"
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
	importLog        = logf.Log.WithName("upload-controller-test")
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
		testPvc := createPvc("testPvc1", "default", map[string]string{AnnPodPhase: string(corev1.PodPending), AnnEndpoint: testEndPoint, AnnSource: SourceHTTP}, nil)
		Expect(shouldReconcilePVC(testPvc)).To(BeTrue())
	})

	It("Should NOT be interesting if complete, and endpoint and source is set", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{AnnPodPhase: string(corev1.PodSucceeded), AnnEndpoint: testEndPoint, AnnSource: SourceHTTP}, nil)
		Expect(shouldReconcilePVC(testPvc)).To(BeFalse())
	})

	It("Should be interesting if NOT complete, and endpoint missing and source is set", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{AnnPodPhase: string(corev1.PodRunning), AnnSource: SourceHTTP}, nil)
		Expect(shouldReconcilePVC(testPvc)).To(BeTrue())
	})

	It("Should be interesting if NOT complete, and endpoint set and source is missing", func() {
		testPvc := createPvc("testPvc1", "default", map[string]string{AnnPodPhase: string(corev1.PodPending), AnnEndpoint: testEndPoint}, nil)
		Expect(shouldReconcilePVC(testPvc)).To(BeTrue())
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
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should return success if no PVC can be found, due to it not existing", func() {
		reconciler = createImportReconciler()
		_, err := reconciler.Reconcile(reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should return success if no PVC can be found due to not existing in passed namespace", func() {
		reconciler = createImportReconciler(createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint}, nil))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "invalid"}})
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should succeed and be marked complete, if creating a block PVC with source none", func() {
		reconciler = createImportReconciler(createBlockPvc("testPvc1", "block", map[string]string{AnnSource: SourceNone}, nil))
		_, err := reconciler.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "block"}})
		Expect(err).ToNot(HaveOccurred())
		resultPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "block"}, resultPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resultPvc.GetAnnotations()[AnnPodPhase]).To(BeEquivalentTo(corev1.PodSucceeded))
	})

	It("should do nothing and not error, if a PVC that is completed is passed", func() {
		orgPvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodSucceeded)}, nil)
		orgPvc.TypeMeta.APIVersion = "v1"
		orgPvc.TypeMeta.Kind = "PersistentVolumeClaim"
		reconciler = createImportReconciler(orgPvc)
		_, err := reconciler.Reconcile(reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(reflect.DeepEqual(orgPvc, resPvc)).To(BeTrue())
	})

	It("Should create a POD if a PVC with all needed annotations is passed", func() {
		reconciler = createImportReconciler(createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint}, nil))
		_, err := reconciler.Reconcile(reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		pod := &corev1.Pod{}
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, pod)
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

	It("Should create a POD if a PVC with all needed annotations is passed, but not set fsgroup if not kubevirt contenttype", func() {
		reconciler = createImportReconciler(createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnContentType: string(cdiv1.DataVolumeArchive)}, nil))
		_, err := reconciler.Reconcile(reconcile.Request{})
		Expect(err).ToNot(HaveOccurred())
		pod := &corev1.Pod{}
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, pod)
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
		_, err := reconciler.Reconcile(reconcile.Request{})
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
		}
		reconciler = createImportReconciler(pvc, pod)
		resPod := &corev1.Pod{}
		err := reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.updatePvcFromPod(pvc, pod, reconciler.Log)
		Expect(err).ToNot(HaveOccurred())
		By("Checking import successful event recorded")
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring("Import Successful"))
		By("Checking pvc phase has been updated")
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[AnnPodPhase]).To(BeEquivalentTo(corev1.PodSucceeded))
		By("Checking pod has been deleted")
		resPod = &corev1.Pod{}
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("Should update the PVC status to running, if pod is running", func() {
		pvc := createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending)}, nil)
		pod := createImporterTestPod(pvc, "testPvc1", nil)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
		}
		reconciler = createImportReconciler(pvc, pod)
		resPod := &corev1.Pod{}
		err := reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).ToNot(HaveOccurred())
		err = reconciler.updatePvcFromPod(pvc, pod, reconciler.Log)
		Expect(err).ToNot(HaveOccurred())
		By("Checking pvc phase has been updated")
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[AnnPodPhase]).To(BeEquivalentTo(corev1.PodRunning))
		Expect(resPvc.GetAnnotations()[AnnImportPod]).To(Equal(pod.Name))
		By("Checking pod has NOT been deleted")
		resPod = &corev1.Pod{}
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "importer-testPvc1", Namespace: "default"}, resPod)
		Expect(err).ToNot(HaveOccurred())
		By("Making sure the label has been added")
		Expect(resPvc.GetLabels()[common.CDILabelKey]).To(Equal(common.CDILabelValue))
	})

	It("Should create scratch PVC, if pod is pending and PVC is marked with scratch", func() {
		pvc := createPvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending), AnnRequiresScratch: "true"}, nil)
		pod := createImporterTestPod(pvc, "testPvc1", nil)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodPending,
		}
		reconciler = createImportReconciler(pvc, pod)
		err := reconciler.updatePvcFromPod(pvc, pod, reconciler.Log)
		Expect(err).ToNot(HaveOccurred())
		By("Checking scratch PVC has been created")
		// Once all controllers are converted, we will use the runtime lib client instead of client-go and retrieval needs to change here.
		scratchPvc, err := reconciler.K8sClient.CoreV1().PersistentVolumeClaims("default").Get("testPvc1-scratch", metav1.GetOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(scratchPvc.Spec.Resources).To(Equal(pvc.Spec.Resources))

		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[AnnImportPod]).To(Equal(pod.Name))
	})

	// TODO: Update me to stay in progress if we were in progress already, its a pod failure and it will get restarted.
	It("Should update phase on PVC, if pod exited with error state that is NOT scratchspace exit", func() {
		pvc := createPvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodRunning)}, nil)
		pod := createImporterTestPod(pvc, "testPvc1", nil)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodFailed,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Message:  "I went poof",
						},
					},
				},
			},
		}
		reconciler = createImportReconciler(pvc, pod)
		err := reconciler.updatePvcFromPod(pvc, pod, reconciler.Log)
		Expect(err).ToNot(HaveOccurred())
		By("Checking pvc phase has been updated")
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(resPvc.GetAnnotations()[AnnPodPhase]).To(BeEquivalentTo(corev1.PodFailed))
		Expect(resPvc.GetAnnotations()[AnnImportPod]).To(Equal(pod.Name))
		By("Checking error event recorded")
		event := <-reconciler.recorder.(*record.FakeRecorder).Events
		Expect(event).To(ContainSubstring("I went poof"))
	})

	It("Should update phase on PVC, if pod exited with error state that is scratchspace exit", func() {
		pvc := createPvcInStorageClass("testPvc1", "default", &testStorageClass, map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodRunning)}, nil)
		pod := createImporterTestPod(pvc, "testPvc1", nil)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: common.ScratchSpaceNeededExitCode,
							Message:  "scratch space needed",
						},
					},
				},
			},
		}
		reconciler = createImportReconciler(pvc, pod)
		err := reconciler.updatePvcFromPod(pvc, pod, reconciler.Log)
		Expect(err).ToNot(HaveOccurred())
		By("Checking pvc phase has been updated")
		resPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.Client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, resPvc)
		Expect(err).ToNot(HaveOccurred())
		By("Verifying that the phase hasn't changed")
		Expect(resPvc.GetAnnotations()[AnnPodPhase]).To(BeEquivalentTo(corev1.PodRunning))
		Expect(resPvc.GetAnnotations()[AnnImportPod]).To(Equal(pod.Name))
		// No scratch space because the pod is not in pending.
	})
})

var _ = Describe("Create Importer Pod", func() {
	var scratchPvcName = "scratchPvc"

	table.DescribeTable("should", func(pvc *corev1.PersistentVolumeClaim, scratchPvcName *string) {
		reconciler := createImportReconciler(pvc)
		podEnvVar := &importPodEnvVar{
			ep:            "",
			secretName:    "",
			source:        "",
			contentType:   "",
			imageSize:     "1G",
			certConfigMap: "",
			diskID:        "",
			insecureTLS:   false,
		}
		pod, err := createImporterPod(reconciler.Log, reconciler.Client, reconciler.CdiClient, testImage, "5", testPullPolicy, podEnvVar, pvc, scratchPvcName)
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
	},
		table.Entry("should create pod with file system volume mode", createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending)}, nil), nil),
		table.Entry("should create pod with block volume mode", createBlockPvc("testBlockPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending)}, nil), nil),
		table.Entry("should create pod with file system volume mode and scratchspace", createPvc("testPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending)}, nil), &scratchPvcName),
		table.Entry("should create pod with block volume mode and scratchspace", createBlockPvc("testBlockPvc1", "default", map[string]string{AnnEndpoint: testEndPoint, AnnPodPhase: string(corev1.PodPending)}, nil), &scratchPvcName),
	)
})

var _ = Describe("Import test env", func() {
	const mockUID = "1111-1111-1111-1111"

	It("Should create import env", func() {
		testEnvVar := &importPodEnvVar{"myendpoint", "mysecret", SourceHTTP, string(cdiv1.DataVolumeKubeVirt), "1G", "", "", false}
		Expect(reflect.DeepEqual(makeImportEnv(testEnvVar, mockUID), createImportTestEnv(testEnvVar, mockUID))).To(BeTrue())
	})
})

func createImportReconciler(objects ...runtime.Object) *ImportReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)

	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	objs = append(objs, cdiConfig)
	cdifakeclientset := cdifake.NewSimpleClientset(cdiConfig)
	k8sfakeclientset := k8sfake.NewSimpleClientset(createStorageClass(testStorageClass, nil))

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	rec := record.NewFakeRecorder(1)
	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &ImportReconciler{
		Client:    cl,
		Scheme:    s,
		Log:       importLog,
		recorder:  rec,
		CdiClient: cdifakeclientset,
		K8sClient: k8sfakeclientset,
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
			Value: string(uid),
		},
		{
			Name:  common.InsecureTLSVar,
			Value: strconv.FormatBool(podEnvVar.insecureTLS),
		},
		{
			Name:  common.ImporterDiskID,
			Value: podEnvVar.diskID,
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
	contentType := getContentType(pvc)
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
