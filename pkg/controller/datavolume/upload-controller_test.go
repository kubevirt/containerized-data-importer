/*
Copyright 2022 The CDI Authors.

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

package datavolume

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller/common"
	featuregates "kubevirt.io/containerized-data-importer/pkg/feature-gates"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	dvUploadLog = logf.Log.WithName("datavolume-upload-controller-test")
)

var _ = Describe("All DataVolume Tests", func() {
	var (
		reconciler *UploadReconciler
	)
	AfterEach(func() {
		if reconciler != nil {
			reconciler = nil
		}
	})

	var _ = Describe("Reconcile Datavolume status", func() {
		DescribeTable("DV phase", func(testDv runtime.Object, current, expected cdiv1.DataVolumePhase, pvcPhase corev1.PersistentVolumeClaimPhase, podPhase corev1.PodPhase, ann, expectedEvent string, extraAnnotations ...string) {
			scName := "testpvc"
			sc := CreateStorageClassWithProvisioner(scName, map[string]string{AnnDefaultStorageClass: "true"}, map[string]string{}, "csi-plugin")
			storageProfile := createStorageProfile(scName, nil, BlockMode)

			r := createUploadReconciler(testDv, sc, storageProfile)
			dvPhaseTest(r.ReconcilerBase, r.updateStatusPhase, testDv, current, expected, pvcPhase, podPhase, ann, expectedEvent, extraAnnotations...)
		},
			Entry("should switch to scheduled for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.UploadScheduled, corev1.ClaimBound, corev1.PodPending, AnnUploadRequest, "Upload into test-dv scheduled", AnnPriorityClassName, "p0-upload"),
			Entry("should switch to uploadready for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.UploadReady, corev1.ClaimBound, corev1.PodRunning, AnnUploadRequest, "Upload into test-dv ready", AnnPodReady, "true", AnnPriorityClassName, "p0-upload"),
			Entry("should stay the same for upload after pod fails", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.UploadScheduled, corev1.ClaimBound, corev1.PodFailed, AnnUploadRequest, "Upload into test-dv failed", AnnPriorityClassName, "p0-upload"),
			Entry("should switch to failed on claim lost for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.Failed, corev1.ClaimLost, corev1.PodFailed, AnnUploadRequest, "PVC test-dv lost", AnnPriorityClassName, "p0-upload"),
			Entry("should switch to succeeded for upload", newUploadDataVolume("test-dv"), cdiv1.Pending, cdiv1.Succeeded, corev1.ClaimBound, corev1.PodSucceeded, AnnUploadRequest, "Successfully uploaded into test-dv", AnnPriorityClassName, "p0-upload"),
		)
	})
})

func createUploadReconciler(objects ...runtime.Object) *UploadReconciler {
	cdiConfig := MakeEmptyCDIConfigSpec(common.ConfigName)
	cdiConfig.Status = cdiv1.CDIConfigStatus{
		ScratchSpaceStorageClass: testStorageClass,
	}
	cdiConfig.Spec.FeatureGates = []string{featuregates.HonorWaitForFirstConsumer}

	objs := []runtime.Object{}
	objs = append(objs, objects...)
	objs = append(objs, cdiConfig)

	return createUploadReconcilerWithoutConfig(objs...)
}

func createUploadReconcilerWithoutConfig(objects ...runtime.Object) *UploadReconciler {
	objs := []runtime.Object{}
	objs = append(objs, objects...)

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	cdiv1.AddToScheme(s)
	snapshotv1.AddToScheme(s)
	extv1.AddToScheme(s)

	objs = append(objs, MakeEmptyCDICR())

	// Create a fake client to mock API calls.
	cl := fake.NewFakeClientWithScheme(s, objs...)

	rec := record.NewFakeRecorder(10)

	// Create a ReconcileMemcached object with the scheme and fake client.
	r := &UploadReconciler{
		ReconcilerBase: ReconcilerBase{
			client:       cl,
			scheme:       s,
			log:          dvUploadLog,
			recorder:     rec,
			featureGates: featuregates.NewFeatureGates(cl),
			installerLabels: map[string]string{
				common.AppKubernetesPartOfLabel:  "testing",
				common.AppKubernetesVersionLabel: "v0.0.0-tests",
			},
		},
	}
	r.Reconciler = r
	return r
}
