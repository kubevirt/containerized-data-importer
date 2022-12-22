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
	"strings"
	"time"

	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"

	//cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	"kubevirt.io/containerized-data-importer/pkg/util/naming"
)

const (
	uploadRequestAnnotation = "cdi.kubevirt.io/storage.upload.target"
	podPhaseAnnotation      = "cdi.kubevirt.io/storage.pod.phase"
	podReadyAnnotation      = "cdi.kubevirt.io/storage.pod.ready"
	cloneRequestAnnotation  = "k8s.io/CloneRequest"
)

var (
	uploadLog = logf.Log.WithName("upload-controller-test")
)

var _ = Describe("Upload controller reconcile loop", func() {

	It("Should return nil and not create a pod, if pvc can not be found", func() {
		reconciler := createUploadReconciler()
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		podList := &corev1.PodList{}
		err = reconciler.client.List(context.TODO(), podList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(podList.Items)).To(Equal(0))
	})

	It("Should return nil and not create a pod, if neither upload nor clone annotations exist", func() {
		reconciler := createUploadReconciler(cc.CreatePvc("testPvc1", "default", map[string]string{}, nil))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		podList := &corev1.PodList{}
		err = reconciler.client.List(context.TODO(), podList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(podList.Items)).To(Equal(0))
	})

	It("Should requeue and not create a pod if target pvc in use", func() {
		pvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnUploadRequest: ""}, nil)
		pod := podUsingPVC(pvc, false)
		reconciler := createUploadReconciler(pvc, pod)
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
			if strings.Contains(event, "UploadTargetInUse") {
				found = true
			}
		}
		Expect(found).To(BeTrue())
	})

	It("Should return error and not create a pod if both upload and clone annotations exist", func() {
		reconciler := createUploadReconciler(cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnUploadRequest: "", cc.AnnCloneRequest: ""}, nil))
		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("PVC has both clone and upload annotations"))
		podList := &corev1.PodList{}
		err = reconciler.client.List(context.TODO(), podList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(podList.Items)).To(Equal(0))
	})

	It("Should return nil and remove any service and pod, if neither upload nor clone annotations exist", func() {
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{}, nil)
		reconciler := createUploadReconciler(testPvc,
			createUploadPod(testPvc),
			createUploadService(testPvc),
		)
		By("Verifying the pod and service exists")
		uploadPod := &corev1.Pod{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: createUploadResourceName("testPvc1"), Namespace: "default"}, uploadPod)
		Expect(err).ToNot(HaveOccurred())
		Expect(uploadPod.Name).To(Equal(createUploadResourceName(testPvc.Name)))

		uploadService := &corev1.Service{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: createUploadResourceName("testPvc1"), Namespace: "default"}, uploadService)
		Expect(err).ToNot(HaveOccurred())
		Expect(uploadService.Name).To(Equal(createUploadResourceName(testPvc.Name)))

		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying the pod and service no longer exist")
		podList := &corev1.PodList{}
		err = reconciler.client.List(context.TODO(), podList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(podList.Items)).To(Equal(0))

		serviceList := &corev1.ServiceList{}
		err = reconciler.client.List(context.TODO(), serviceList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(serviceList.Items)).To(Equal(0))

	})

	It("Should return nil and remove any service and pod if succeeded", func() {
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnUploadRequest: "", cc.AnnPodPhase: string(corev1.PodSucceeded)}, nil)
		reconciler := createUploadReconciler(testPvc,
			createUploadPod(testPvc),
			createUploadService(testPvc),
		)
		By("Verifying the pod and service exists")
		uploadPod := &corev1.Pod{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: createUploadResourceName("testPvc1"), Namespace: "default"}, uploadPod)
		Expect(err).ToNot(HaveOccurred())
		Expect(uploadPod.Name).To(Equal(createUploadResourceName(testPvc.Name)))

		uploadService := &corev1.Service{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: createUploadResourceName("testPvc1"), Namespace: "default"}, uploadService)
		Expect(err).ToNot(HaveOccurred())
		Expect(uploadService.Name).To(Equal(createUploadResourceName(testPvc.Name)))

		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying the pod and service no longer exist")
		podList := &corev1.PodList{}
		err = reconciler.client.List(context.TODO(), podList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(podList.Items)).To(Equal(0))

		serviceList := &corev1.ServiceList{}
		err = reconciler.client.List(context.TODO(), serviceList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(serviceList.Items)).To(Equal(0))

	})

	It("Should return nil and remove any service and pod if pvc marked for deletion", func() {
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnUploadRequest: "", cc.AnnPodPhase: string(corev1.PodPending)}, nil)
		now := metav1.NewTime(time.Now())
		testPvc.DeletionTimestamp = &now
		reconciler := createUploadReconciler(testPvc,
			createUploadPod(testPvc),
			createUploadService(testPvc),
		)
		By("Verifying the pod and service exists")
		uploadPod := &corev1.Pod{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: createUploadResourceName("testPvc1"), Namespace: "default"}, uploadPod)
		Expect(err).ToNot(HaveOccurred())
		Expect(uploadPod.Name).To(Equal(createUploadResourceName(testPvc.Name)))

		uploadService := &corev1.Service{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: createUploadResourceName("testPvc1"), Namespace: "default"}, uploadService)
		Expect(err).ToNot(HaveOccurred())
		Expect(uploadService.Name).To(Equal(createUploadResourceName(testPvc.Name)))

		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying the pod and service no longer exist")
		podList := &corev1.PodList{}
		err = reconciler.client.List(context.TODO(), podList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(podList.Items)).To(Equal(0))

		serviceList := &corev1.ServiceList{}
		err = reconciler.client.List(context.TODO(), serviceList, &client.ListOptions{})
		Expect(err).ToNot(HaveOccurred())
		Expect(len(serviceList.Items)).To(Equal(0))

	})

	It("Should return err and not clone if source and content type do not match", func() {
		storageClassName := "test"
		testPvc := cc.CreatePvcInStorageClass("testPvc1", "default", &storageClassName, map[string]string{cloneRequestAnnotation: "default/sourcePvc"}, nil, corev1.ClaimBound)
		sourcePvc := cc.CreatePvcInStorageClass("sourcePvc", "default", &storageClassName, map[string]string{cc.AnnContentType: "archive"}, nil, corev1.ClaimBound)
		vm := corev1.PersistentVolumeBlock
		sourcePvc.Spec.VolumeMode = &vm
		reconciler := createUploadReconciler(testPvc, sourcePvc)

		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("source contentType (archive) and target contentType (kubevirt) do not match"))
	})

	It("Should return err and not clone if source and target volume modes do not match, and content type is not kubevirt", func() {
		storageClassName := "test"
		testPvc := cc.CreatePvcInStorageClass("testPvc1", "default", &storageClassName, map[string]string{cc.AnnContentType: "archive", cloneRequestAnnotation: "default/sourcePvc"}, nil, corev1.ClaimBound)
		sourcePvc := cc.CreatePvcInStorageClass("sourcePvc", "default", &storageClassName, map[string]string{cc.AnnContentType: "archive"}, nil, corev1.ClaimBound)
		vm := corev1.PersistentVolumeBlock
		sourcePvc.Spec.VolumeMode = &vm
		reconciler := createUploadReconciler(testPvc, sourcePvc)

		_, err := reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Source and target volume modes do not match, and content type is not kubevirt"))
	})

	It("Should return nil and create a pod and service when a clone pvc", func() {
		testPvc := cc.CreatePvc("testPvc1", "default", map[string]string{cc.AnnCloneRequest: "default/testPvc2", AnnUploadPod: createUploadResourceName("testPvc1"), cc.AnnPriorityClassName: "p0"}, nil)
		testPvcSource := cc.CreatePvc("testPvc2", "default", map[string]string{}, nil)
		reconciler := createUploadReconciler(testPvc, testPvcSource)
		By("Verifying the pod and service do not exist")
		uploadPod := &corev1.Pod{}
		err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: createUploadResourceName("testPvc1"), Namespace: "default"}, uploadPod)
		Expect(err).To(HaveOccurred())

		uploadService := &corev1.Service{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: createUploadResourceName("testPvc1"), Namespace: "default"}, uploadService)
		Expect(err).To(HaveOccurred())

		_, err = reconciler.Reconcile(context.TODO(), reconcile.Request{NamespacedName: types.NamespacedName{Name: "testPvc1", Namespace: "default"}})
		Expect(err).ToNot(HaveOccurred())
		By("Verifying the pod and service now exist")
		uploadPod = &corev1.Pod{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: createUploadResourceName("testPvc1"), Namespace: "default"}, uploadPod)
		Expect(err).ToNot(HaveOccurred())
		Expect(uploadPod.Name).To(Equal(createUploadResourceName(testPvc.Name)))
		Expect(uploadPod.Spec.PriorityClassName).To(Equal("p0"))
		Expect(uploadPod.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))

		uploadService = &corev1.Service{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: createUploadResourceName("testPvc1"), Namespace: "default"}, uploadService)
		Expect(err).ToNot(HaveOccurred())
		Expect(uploadService.Name).To(Equal(createUploadResourceName(testPvc.Name)))
		Expect(uploadService.Labels[common.AppKubernetesPartOfLabel]).To(Equal("testing"))

		secret := &corev1.Secret{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: createUploadResourceName("testPvc1"), Namespace: "default"}, secret)
		Expect(err).ToNot(HaveOccurred())
		Expect(secret.Name).To(Equal(createUploadResourceName(testPvc.Name)))
		Expect(secret.OwnerReferences).To(HaveLen(1))
		or := secret.OwnerReferences[0]
		Expect(or.Kind).To(Equal("Pod"))
		Expect(or.Name).To(Equal(uploadPod.Name))
		Expect(or.UID).To(Equal(uploadPod.UID))
	})
})

var _ = Describe("reconcilePVC loop", func() {
	testPvcName := "testPvc1"
	uploadResourceName := "uploader" //createUploadResourceName(testPvcName)

	Context("Is clone", func() {
		isClone := true

		It("Should create the pod name", func() {
			testPvc := cc.CreatePvc(testPvcName, "default", map[string]string{cc.AnnCloneRequest: "default/testPvc2"}, nil)
			testPvcSource := cc.CreatePvc("testPvc2", "default", map[string]string{}, nil)
			reconciler := createUploadReconciler(testPvc, testPvcSource)
			By("Verifying the pod and service do not exist")
			uploadPod := &corev1.Pod{}
			err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadResourceName, Namespace: "default"}, uploadPod)
			Expect(err).To(HaveOccurred())

			uploadService := &corev1.Service{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: naming.GetServiceNameFromResourceName(uploadResourceName), Namespace: "default"}, uploadService)
			Expect(err).To(HaveOccurred())

			_, err = reconciler.reconcilePVC(reconciler.log, testPvc, isClone)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying the pod name annotation")

			resultPvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: testPvcName, Namespace: "default"}, resultPvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(resultPvc.GetAnnotations()[AnnUploadPod]).To(Equal("cdi-upload-testPvc1"))
			Expect(resultPvc.GetAnnotations()[cc.AnnPodPhase]).To(BeEquivalentTo(uploadPod.Status.Phase))
		})

		It("Should create the service and pod with passed annotations", func() {
			testPvc := cc.CreatePvc(testPvcName, "default", map[string]string{cc.AnnCloneRequest: "default/testPvc2", AnnUploadPod: uploadResourceName, cc.AnnPodNetwork: "net1"}, nil)
			testPvcSource := cc.CreatePvc("testPvc2", "default", map[string]string{}, nil)
			reconciler := createUploadReconciler(testPvc, testPvcSource)
			By("Verifying the pod and service do not exist")
			uploadPod := &corev1.Pod{}
			err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadResourceName, Namespace: "default"}, uploadPod)
			Expect(err).To(HaveOccurred())

			uploadService := &corev1.Service{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: naming.GetServiceNameFromResourceName(uploadResourceName), Namespace: "default"}, uploadService)
			Expect(err).To(HaveOccurred())

			_, err = reconciler.reconcilePVC(reconciler.log, testPvc, isClone)
			Expect(err).ToNot(HaveOccurred())

			By("Verifying the pod and service now exist")
			uploadPod = &corev1.Pod{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadResourceName, Namespace: "default"}, uploadPod)
			Expect(err).ToNot(HaveOccurred())
			Expect(uploadPod.Name).To(Equal(uploadResourceName))
			Expect(uploadPod.Labels[common.UploadTargetLabel]).To(Equal(string(testPvc.UID)))
			Expect(uploadPod.GetAnnotations()[cc.AnnPodNetwork]).To(Equal("net1"))
			Expect(uploadPod.GetAnnotations()[cc.AnnPodSidecarInjection]).To(Equal(cc.AnnPodSidecarInjectionDefault))

			uploadService = &corev1.Service{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: naming.GetServiceNameFromResourceName(uploadResourceName), Namespace: "default"}, uploadService)
			Expect(err).ToNot(HaveOccurred())
			Expect(uploadService.Name).To(Equal(uploadResourceName))

			resultPvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: testPvcName, Namespace: "default"}, resultPvc)
			Expect(err).ToNot(HaveOccurred())
			Expect(resultPvc.GetAnnotations()[cc.AnnPodPhase]).To(BeEquivalentTo(uploadPod.Status.Phase))
			Expect(resultPvc.GetAnnotations()[cc.AnnPodReady]).To(Equal("false"))

			secret := &corev1.Secret{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadPod.Name, Namespace: "default"}, secret)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should error if a POD with the same name exists, but is not owned by the PVC, if a PVC with all needed annotations is passed", func() {
			pod := &corev1.Pod{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      uploadResourceName,
					Namespace: "default",
				},
			}
			testPvc := cc.CreatePvc(testPvcName, "default", map[string]string{cc.AnnCloneRequest: "default/testPvc2", AnnUploadPod: uploadResourceName}, nil)
			testPvcSource := cc.CreatePvc("testPvc2", "default", map[string]string{}, nil)
			reconciler := createUploadReconciler(testPvc, testPvcSource, pod)

			_, err := reconciler.reconcilePVC(reconciler.log, testPvc, isClone)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("uploader pod not controlled by pvc testPvc1"))

			uploadService := &corev1.Service{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: naming.GetServiceNameFromResourceName(uploadResourceName), Namespace: "default"}, uploadService)
			Expect(err).To(HaveOccurred())

			secret := &corev1.Secret{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadResourceName, Namespace: "default"}, secret)
			Expect(err).To(HaveOccurred())
		})

		It("Should error if a Service with the same name exists, but is not owned by the PVC, if a PVC with all needed annotations is passed", func() {
			svcName := naming.GetServiceNameFromResourceName(uploadResourceName)
			service := &corev1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      svcName,
					Namespace: "default",
					//Annotations: map[string]string{
					//	annCreatedByUpload: "yes",
					//},
					Labels: map[string]string{
						"app":             "containerized-data-importer",
						"cdi.kubevirt.io": "cdi-upload-server",
					},
				},
			}

			testPvc := cc.CreatePvc(testPvcName, "default", map[string]string{cc.AnnCloneRequest: "default/testPvc2", AnnUploadPod: uploadResourceName}, nil)
			testPvcSource := cc.CreatePvc("testPvc2", "default", map[string]string{}, nil)
			reconciler := createUploadReconciler(testPvc, testPvcSource, service)

			_, err := reconciler.reconcilePVC(reconciler.log, testPvc, isClone)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("uploader service not controlled by pvc testPvc1"))
		})
	})

	Context("Is upload", func() {
		isClone := false

		It("Should create the service and pod", func() {
			testPvc := cc.CreatePvc(testPvcName, "default", map[string]string{cc.AnnUploadRequest: "", AnnUploadPod: uploadResourceName}, nil)
			reconciler := createUploadReconciler(testPvc)
			By("Verifying the pod and service do not exist")
			uploadPod := &corev1.Pod{}
			err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadResourceName, Namespace: "default"}, uploadPod)
			Expect(err).To(HaveOccurred())

			uploadService := &corev1.Service{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadResourceName, Namespace: "default"}, uploadService)
			Expect(err).To(HaveOccurred())

			_, err = reconciler.reconcilePVC(reconciler.log, testPvc, isClone)
			Expect(err).ToNot(HaveOccurred())
			By("Verifying the pod and service now exist")
			uploadPod = &corev1.Pod{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadResourceName, Namespace: "default"}, uploadPod)
			Expect(err).ToNot(HaveOccurred())
			Expect(uploadPod.Name).To(Equal(uploadResourceName))

			uploadService = &corev1.Service{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadResourceName, Namespace: "default"}, uploadService)
			Expect(err).ToNot(HaveOccurred())
			Expect(uploadService.Name).To(Equal(uploadResourceName))

			scratchPvc := &corev1.PersistentVolumeClaim{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1-scratch", Namespace: "default"}, scratchPvc)
			Expect(err).ToNot(HaveOccurred())

			secret := &corev1.Secret{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadResourceName, Namespace: "default"}, secret)
			Expect(err).ToNot(HaveOccurred())
		})

		table.DescribeTable("should pass correct crypto config to created pod", func(profile *ocpconfigv1.TLSSecurityProfile) {
			testPvc := cc.CreatePvc(testPvcName, "default", map[string]string{cc.AnnUploadRequest: "", AnnUploadPod: uploadResourceName}, nil)
			reconciler := createUploadReconciler(testPvc)
			cdiConfig := &cdiv1.CDIConfig{}
			err := reconciler.client.Get(context.TODO(), types.NamespacedName{Name: common.ConfigName}, cdiConfig)
			Expect(err).ToNot(HaveOccurred())
			profileType := ocpconfigv1.TLSProfileIntermediateType
			if profile != nil {
				profileType = profile.Type
				cdiConfig.Spec.TLSSecurityProfile = profile
				err = reconciler.client.Update(context.TODO(), cdiConfig)
			}

			_, err = reconciler.reconcilePVC(reconciler.log, testPvc, isClone)
			Expect(err).ToNot(HaveOccurred())
			By("Verifying the pod exists and holds our env vars")
			uploadPod := &corev1.Pod{}
			err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: uploadResourceName, Namespace: "default"}, uploadPod)
			Expect(err).ToNot(HaveOccurred())
			Expect(uploadPod.Name).To(Equal(uploadResourceName))

			foundMinVersionEnvVar := false
			for _, envVar := range uploadPod.Spec.Containers[0].Env {
				if envVar.Name == common.MinVersionTLSVar {
					Expect(envVar.Value).To(Equal(string(ocpconfigv1.TLSProfiles[profileType].MinTLSVersion)))
					foundMinVersionEnvVar = true
				}
			}
			Expect(foundMinVersionEnvVar).To(BeTrue())
			foundCiphersEnvVar := false
			for _, envVar := range uploadPod.Spec.Containers[0].Env {
				if envVar.Name == common.CiphersTLSVar {
					Expect(envVar.Value).To(Equal(strings.Join(ocpconfigv1.TLSProfiles[profileType].Ciphers, ",")))
					foundCiphersEnvVar = true
				}
			}
			Expect(foundCiphersEnvVar).To(BeTrue())
		},
			table.Entry("no profile set", nil),
			table.Entry("'Old' profile set", &ocpconfigv1.TLSSecurityProfile{Type: ocpconfigv1.TLSProfileOldType, Old: &ocpconfigv1.OldTLSProfile{}}),
		)
	})
})

var _ = Describe("Update PVC", func() {

	It("Should update cc.AnnPodRestarts on pvc from upload pod restarts", func() {
		testPvc := cc.CreatePvc("testPvc1", "default",
			map[string]string{
				cc.AnnUploadRequest: "",
				cc.AnnPodPhase:      string(corev1.PodPending),
				cc.AnnPodRestarts:   "1"}, nil)
		pod := createUploadPod(testPvc)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					RestartCount: 2,
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
		reconciler := createUploadReconciler(testPvc, pod, createUploadService(testPvc))

		_, err := reconciler.reconcilePVC(reconciler.log, testPvc, false)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying the pvc has restarts updated")
		actualPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, actualPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualPvc.Annotations[cc.AnnPodRestarts]).To(Equal("2"))
		Expect(actualPvc.GetAnnotations()[cc.AnnRunningCondition]).To(Equal("false"))
		Expect(actualPvc.GetAnnotations()[cc.AnnRunningConditionMessage]).To(BeEmpty())
		Expect(actualPvc.GetAnnotations()[cc.AnnRunningConditionReason]).To(BeEmpty())
		Expect(actualPvc.GetAnnotations()[cc.AnnBoundCondition]).To(Equal("false"))
		Expect(actualPvc.GetAnnotations()[cc.AnnBoundConditionMessage]).To(Equal("Creating scratch space"))
		Expect(actualPvc.GetAnnotations()[cc.AnnBoundConditionReason]).To(Equal(creatingScratch))
	})

	It("Should not update AnnPodRestarts on pvc from pod if pod has lower restart count value ", func() {
		testPvc := cc.CreatePvc("testPvc1", "default",
			map[string]string{
				cc.AnnUploadRequest: "",
				cc.AnnPodPhase:      string(corev1.PodRunning),
				cc.AnnPodRestarts:   "3"},
			nil)
		pod := createUploadPod(testPvc)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					RestartCount: 2,
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
		reconciler := createUploadReconciler(testPvc, pod, createUploadService(testPvc))

		_, err := reconciler.reconcilePVC(reconciler.log, testPvc, false)
		Expect(err).ToNot(HaveOccurred())

		By("Verifying the pvc has original restart count")
		actualPvc := &corev1.PersistentVolumeClaim{}
		err = reconciler.client.Get(context.TODO(), types.NamespacedName{Name: "testPvc1", Namespace: "default"}, actualPvc)
		Expect(err).ToNot(HaveOccurred())
		Expect(actualPvc.Annotations[cc.AnnPodRestarts]).To(Equal("3"))
		Expect(actualPvc.GetAnnotations()[cc.AnnRunningCondition]).To(Equal("false"))
		Expect(actualPvc.GetAnnotations()[cc.AnnRunningConditionMessage]).To(BeEmpty())
		Expect(actualPvc.GetAnnotations()[cc.AnnRunningConditionReason]).To(BeEmpty())
		Expect(actualPvc.GetAnnotations()[cc.AnnBoundCondition]).To(Equal("false"))
		Expect(actualPvc.GetAnnotations()[cc.AnnBoundConditionMessage]).To(Equal("Creating scratch space"))
		Expect(actualPvc.GetAnnotations()[cc.AnnBoundConditionReason]).To(Equal(creatingScratch))
	})
})

var _ = Describe("updateUploadAnnotations", func() {
	It("Should update the annotations", func() {
		testPvc := cc.CreatePvc("testPvc", "default", map[string]string{}, nil)
		pod := createUploadPod(testPvc)
		pod.Status = corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					RestartCount: 1,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		}

		pvcCopy := testPvc.DeepCopy()

		updateUploadAnnotations(testPvc, pvcCopy.Annotations, pod, false)
		Expect(pvcCopy.Annotations[cc.AnnPodRestarts]).To(Equal("1"))
		Expect(pvcCopy.GetAnnotations()[cc.AnnRunningCondition]).To(Equal("true"))
		Expect(pvcCopy.GetAnnotations()[cc.AnnRunningConditionMessage]).To(Equal(""))
		Expect(pvcCopy.GetAnnotations()[cc.AnnRunningConditionReason]).To(Equal(PodRunningReason))
	})
})

func createUploadReconciler(objects ...runtime.Object) *UploadReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cc.MakeEmptyCDICR())
	// Append empty CDIConfig object that normally is created by the reconcile loop
	cdiConfig := cc.MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		DefaultPodResourceRequirements: createDefaultPodResourceRequirements("", "", "", ""),
	}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}

	objs = append(objs, cdiConfig)
	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	rec := record.NewFakeRecorder(10)

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &UploadReconciler{
		client:              cl,
		scheme:              s,
		log:                 uploadLog,
		serverCertGenerator: &fakeCertGenerator{},
		clientCAFetcher:     &fetcher.MemCertBundleFetcher{Bundle: []byte("baz")},
		recorder:            rec,
		featureGates:        featuregates.NewFeatureGates(cl),
		installerLabels: map[string]string{
			common.AppKubernetesPartOfLabel:  "testing",
			common.AppKubernetesVersionLabel: "v0.0.0-tests",
		},
	}
	return r
}

func createUploadPod(pvc *corev1.PersistentVolumeClaim) *corev1.Pod {
	pod := createUploadClonePod(pvc, "client.upload-server.cdi.kubevirt.io")
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: cc.ScratchVolName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvc.Name + "-scratch",
				ReadOnly:  false,
			},
		},
	})
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      cc.ScratchVolName,
		MountPath: "/scratch",
	})
	return pod
}

func createUploadClonePod(pvc *corev1.PersistentVolumeClaim, clientName string) *corev1.Pod {
	name := "cdi-upload-" + pvc.Name
	requestImageSize, _ := cc.GetRequestedImageSize(pvc)

	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pvc.Namespace,
			Annotations: map[string]string{
				annCreatedByUpload: "yes",
			},
			Labels: map[string]string{
				"app":             "containerized-data-importer",
				"cdi.kubevirt.io": "cdi-upload-server",
				"service":         name,
			},
			OwnerReferences: []metav1.OwnerReference{
				MakePVCOwnerReference(pvc),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "cdi-upload-server",
					Image:           "test/myimage",
					ImagePullPolicy: corev1.PullPolicy("Always"),
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      cc.DataVolName,
							MountPath: "/data",
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "TLS_KEY",
							Value: "bar",
						},
						{
							Name:  "TLS_CERT",
							Value: "foo",
						},
						{
							Name:  "CLIENT_CERT",
							Value: "baz",
						},
						{
							Name:  common.UploadImageSize,
							Value: requestImageSize,
						},
						{
							Name:  "CLIENT_NAME",
							Value: clientName,
						},
					},
					Args: []string{"-v=" + "5"},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							HTTPGet: &corev1.HTTPGetAction{
								Path: "/healthz",
								Port: intstr.IntOrString{
									Type:   intstr.Int,
									IntVal: 8080,
								},
							},
						},
						InitialDelaySeconds: 2,
						PeriodSeconds:       5,
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Volumes: []corev1.Volume{
				{
					Name: cc.DataVolName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
							ReadOnly:  false,
						},
					},
				},
			},
		},
	}
	return pod
}

func createUploadService(pvc *corev1.PersistentVolumeClaim) *corev1.Service {
	name := "cdi-upload-" + pvc.Name
	blockOwnerDeletion := true
	isController := true
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pvc.Namespace,
			Annotations: map[string]string{
				annCreatedByUpload: "yes",
			},
			Labels: map[string]string{
				"app":             "containerized-data-importer",
				"cdi.kubevirt.io": "cdi-upload-server",
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
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Protocol: "TCP",
					Port:     443,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8443,
					},
				},
			},
			Selector: map[string]string{
				"service": name,
			},
		},
	}
	return service
}

func createUploadServerCertSecret(pvc *corev1.PersistentVolumeClaim) *corev1.Secret {
	name := "cdi-upload-" + pvc.Name
	pod := createUploadPod(pvc)
	blockOwnerDeletion := true
	isController := true
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: pvc.Namespace,
			Annotations: map[string]string{
				annCreatedByUpload: "yes",
			},
			Labels: map[string]string{
				common.CDILabelKey:       common.CDILabelValue,
				common.CDIComponentLabel: common.UploadServerCDILabel,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "Pod",
					Name:               pod.Name,
					UID:                pod.GetUID(),
					BlockOwnerDeletion: &blockOwnerDeletion,
					Controller:         &isController,
				},
			},
		},
		Data: map[string][]byte{
			"tls.key": []byte("key"),
			"tls.crt": []byte("cert"),
		},
	}
}
