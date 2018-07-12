// +build unit_test

package controller

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	k8stesting "k8s.io/client-go/tools/cache/testing"
	. "kubevirt.io/containerized-data-importer/pkg/common"
	. "kubevirt.io/containerized-data-importer/pkg/controller"
)

type operation int

const (
	opAdd operation = iota
	opUpdate
	opDelete
)

var verboseDebug = fmt.Sprintf("%d", Vdebug)

var _ = Describe("Controller", func() {
	var (
		controller *ImportController
		fakeClient *fake.Clientset
		pvcSource  *k8stesting.FakePVCControllerSource
		podSource  *k8stesting.FakeControllerSource
		stop       chan struct{}
	)

	setUpInformers := func(pvc *v1.PersistentVolumeClaim, pod *v1.Pod, ns string, op operation, stop <-chan struct{}) {
		// Populate the informer caches with test objects
		var err error
		pvc, err = fakeClient.CoreV1().PersistentVolumeClaims(ns).Create(pvc)
		Expect(err).NotTo(HaveOccurred())
		pvcSource.Add(pvc)

		if pod != nil {
			pod, err = fakeClient.CoreV1().Pods(ns).Create(pod)
			Expect(err).NotTo(HaveOccurred())
			podSource.Add(pod)
		}

		pvcInformer := cache.NewSharedIndexInformer(pvcSource, pvc, DEFAULT_RESYNC_PERIOD, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		podInformer := cache.NewSharedIndexInformer(podSource, pod, DEFAULT_RESYNC_PERIOD, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})

		controller = NewImportController(fakeClient, pvcInformer, podInformer, IMPORTER_DEFAULT_IMAGE, DEFAULT_PULL_POLICY, verboseDebug)

		go pvcInformer.Run(stop)
		go podInformer.Run(stop)
		Expect(cache.WaitForCacheSync(stop, pvcInformer.HasSynced)).To(BeTrue())
		Expect(cache.WaitForCacheSync(stop, podInformer.HasSynced)).To(BeTrue())
	}

	BeforeEach(func() {
		By("Setting up new fake kubernetes client")
		fakeClient = fake.NewSimpleClientset()
		stop = make(chan struct{})

		By("Setting up new fake controller sources")
		pvcSource = k8stesting.NewFakePVCControllerSource()
		podSource = k8stesting.NewFakeControllerSource()
	})

	AfterEach(func() {
		By("Stopping informer routines")
		close(stop)
	})

	Context("when parsing pvc annotations", func() {

		type testT struct {
			descr       string
			ns          string
			name        string // name of test pvc
			qops        operation
			annotations map[string]string
			expectError bool
		}

		tests := []testT{
			{
				descr:       "pvc, endpoint, blank ns: controller creates importer pod",
				ns:          "", // blank used originally in these unit tests
				name:        "test-pvc",
				qops:        opAdd,
				annotations: map[string]string{AnnEndpoint: "http://www.google.com"},
				expectError: false,
			},
			{
				descr:       "pvc, endpoint, non-blank ns: controller creates importer pod",
				ns:          "ns-a",
				name:        "test-pvc",
				qops:        opAdd,
				annotations: map[string]string{AnnEndpoint: "http://www.google.com"},
				expectError: false,
			},
			{
				descr:       "pvc, blank endpoint: controller does not create importer pod",
				ns:          "",
				name:        "test-pvc",
				qops:        opAdd,
				annotations: map[string]string{},
				expectError: true,
			},
			{
				descr:       "updated pvc should process",
				ns:          "ns-a",
				name:        "test-pvc",
				qops:        opUpdate,
				annotations: map[string]string{AnnEndpoint: "http://www.google.com"},
				expectError: false,
			},
			{
				descr:       "updated pvc should not process based on annotation AnnImportPod indicating already been processed",
				ns:          "ns-a",
				name:        "test-pvc",
				qops:        opUpdate,
				annotations: map[string]string{AnnEndpoint: "http://www.google.com", AnnImportPod: "importer-test-pvc"},
				expectError: true,
			},
		}
		for _, test := range tests {
			ns := test.ns
			pvcName := test.name
			ops := test.qops
			annotations := test.annotations
			fullname := pvcName
			if len(ns) > 0 {
				fullname = fmt.Sprintf("%s/%s", ns, pvcName)
			}
			exptPod := fmt.Sprintf("importer-%s-", pvcName)
			exptErr := test.expectError

			It(test.descr, func() {
				By(fmt.Sprintf("creating in-mem pvc %q with endpt anno=%q", fullname, annotations))
				pvc := createInMemPVC(ns, pvcName, annotations)
				By("Creating the controller")
				setUpInformers(pvc, nil, ns, ops, stop)
				controller.ProcessNextPvcItem()
				By("checking if importer pod is present")
				pod, err := getImporterPod(fakeClient, ns, exptPod)
				if exptErr {
					By("Expecting no pod created")
					Expect(err).ToNot(BeNil(), fmt.Sprintf("importer pod %s... should not exist\n", exptPod))
				} else {
					By("Expecting pod created")
					Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("importer pod: %v\n", err))
					Expect(pod).ToNot(BeNil(), fmt.Sprintf("importer pod %q missing", exptPod))
					Expect(pod.GenerateName).To(HavePrefix(exptPod))
				}
			})
		}
	})

	Context("when annotating pod status in pvc", func() {
		const (
			ep          = "https://www.google.com"
			pvcName     = "test-pvc"
			ns          = "testing-namespace"
			expectPhase = v1.PodPending
		)

		type test struct {
			desc                     string
			podAnn                   map[string]string
			hasVolume, shouldSucceed bool
		}
		tests := []test{
			{
				desc:          fmt.Sprintf("Should annotate the pod phase in the pvc when the pod has annotation \"%s: \"", AnnCreatedBy),
				podAnn:        map[string]string{AnnCreatedBy: ""},
				hasVolume:     true,
				shouldSucceed: true,
			},
			{
				desc:          fmt.Sprintf("Should do nothing when the pod is missing annotation \"%s: %s\"", AnnCreatedBy, ""),
				podAnn:        map[string]string{},
				hasVolume:     true,
				shouldSucceed: false,
			},
			{
				desc:          "Should do nothing if there is no volume attached to the pod",
				podAnn:        map[string]string{AnnCreatedBy: ""},
				hasVolume:     false,
				shouldSucceed: false,
			},
		}

		var pvcAnn = map[string]string{
			AnnEndpoint: ep,
		}

		for _, t := range tests {
			podAnn := t.podAnn
			ss := t.shouldSucceed
			hasVol := t.hasVolume
			It(t.desc, func() {
				By("Setting up API objects and starting informers")
				pvc := createInMemPVC(ns, pvcName, pvcAnn)
				pod := createInMemPod(ns, pvcName, expectPhase, hasVol, podAnn)
				pod.Status.Phase = expectPhase
				setUpInformers(pvc, pod, ns, opAdd, stop)

				By("Initiating pod phase write to pvc annotation")
				controller.ProcessNextPodItem()
				pvc, err := fakeClient.CoreV1().PersistentVolumeClaims(ns).Get(pvcName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				if ss {
					By("Expecting pod phase annotation in the pvc")
					Expect(pvc.Annotations[AnnPodPhase]).To(Equal(string(expectPhase)))
				} else {
					By("Expecting no pod phase annotation in the pvc")
					Expect(pvc.Annotations[AnnPodPhase]).To(BeEmpty())
				}
			})
		}

	})
})

// return an in-memory pvc using the passed-in namespace, name and the endpoint annotation.
func createInMemPVC(ns, name string, annotations map[string]string) *v1.PersistentVolumeClaim {
	return &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ns,
			Annotations: annotations,
		},
	}
}

// createInMemPod generates a pod with the passed-in values.
func createInMemPod(ns, pvcName string, phase v1.PodPhase, hasVol bool, annotations map[string]string) *v1.Pod {
	var volName string
	if hasVol {
		volName = DataVolName
	}
	genName := fmt.Sprintf("importer-%s-", pvcName)

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    ns,
			GenerateName: genName,
			Name:         fmt.Sprintf("%s1234", genName),
			Annotations:  annotations,
		},
		Spec: v1.PodSpec{
			Volumes: []v1.Volume{
				{
					Name: volName,
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
		Status: v1.PodStatus{
			Phase: phase,
		},
	}
}

// getImporterPod gets the first pod with a generated name equal to the passed-in name.
// Nil is returned if no match is found.
func getImporterPod(fc *fake.Clientset, ns, podName string) (*v1.Pod, error) {
	podList, err := fc.CoreV1().Pods(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "could not list pods in namespace %q", ns)
	}
	if len(podList.Items) == 0 {
		return nil, errors.Errorf("Found 0 pods in namespace %q", ns)
	}
	for i, p := range podList.Items {
		if p.GenerateName == podName {
			return &podList.Items[i], nil
		}
	}
	return nil, errors.Errorf("no pods match %s/%s", ns, podName)
}
