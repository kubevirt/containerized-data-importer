package controller_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sinformers "k8s.io/client-go/informers"
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
	IMPORTER_DEFAULT_IMAGE = "kubevirt/cdi-importer:latest"
)

var verboseDebug = fmt.Sprintf("%d", 3)

var _ = Describe("Import Controller", func() {
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

		pvcInformerFactory := k8sinformers.NewSharedInformerFactory(fakeClient, DefaultResyncPeriod)
		podInformerFactory := k8sinformers.NewSharedInformerFactory(fakeClient, DefaultResyncPeriod)

		pvcInformer := pvcInformerFactory.Core().V1().PersistentVolumeClaims()
		podInformer := podInformerFactory.Core().V1().Pods()

		controller = NewImportController(fakeClient, pvcInformer, podInformer, IMPORTER_DEFAULT_IMAGE, DefaultPullPolicy, verboseDebug)

		go pvcInformerFactory.Start(stop)
		go podInformerFactory.Start(stop)
		Expect(cache.WaitForCacheSync(stop, pvcInformer.Informer().HasSynced)).To(BeTrue())
		Expect(cache.WaitForCacheSync(stop, podInformer.Informer().HasSynced)).To(BeTrue())
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
				descr:       "updated pvc should not process based on annotation AnnPodPhase=Succeeded indicating already been processed",
				ns:          "ns-a",
				name:        "test-pvc",
				qops:        opUpdate,
				annotations: map[string]string{AnnEndpoint: "http://www.google.com", AnnPodPhase: "Succeeded"},
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
			desc          string
			podLabel      map[string]string
			shouldSucceed bool
		}
		tests := []test{
			{
				desc:          fmt.Sprintf("Should annotate the pod phase in the pvc when the pod has label \"%s: \"", LabelImportPvc),
				podLabel:      map[string]string{LabelImportPvc: pvcName},
				shouldSucceed: true,
			},
			{
				desc:          fmt.Sprintf("Should do nothing when the pod is missing label \"%s: %s\"", LabelImportPvc, ""),
				podLabel:      map[string]string{},
				shouldSucceed: false,
			},
		}

		var pvcAnn = map[string]string{
			AnnEndpoint: ep,
		}

		for _, t := range tests {
			podLabel := t.podLabel
			ss := t.shouldSucceed
			It(t.desc, func() {
				By("Setting up API objects and starting informers")
				pvc := createInMemPVC(ns, pvcName, pvcAnn)
				pod := createInMemPod(ns, pvc, expectPhase, podLabel)
				pod.Status.Phase = expectPhase
				setUpInformers(pvc, pod, ns, opAdd, stop)

				By("Initiating pod phase write to pvc annotation")
				controller.ProcessNextPvcItem()
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
			UID:         "1234",
		},
		Spec: v1.PersistentVolumeClaimSpec{
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): resource.MustParse("1G"),
				},
			},
		},
	}
}

// createInMemPod generates a pod with the passed-in values.
func createInMemPod(ns string, pvc *v1.PersistentVolumeClaim, phase v1.PodPhase, labels map[string]string) *v1.Pod {
	volName := DataVolName
	pvcName := pvc.Name
	genName := fmt.Sprintf("importer-%s-", pvcName)

	blockOwnerDeletion := true
	isController := true
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    ns,
			GenerateName: genName,
			Name:         fmt.Sprintf("%s1234", genName),
			Labels:       labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "PersistentVolumeClaim",
					Name:               pvcName,
					UID:                pvc.GetUID(),
					BlockOwnerDeletion: &blockOwnerDeletion,
					Controller:         &isController,
				},
			},
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
	} else if len(podList.Items) > 1 {
		return nil, errors.Errorf("Found > 1 pods in namespace %q", ns)
	}

	for i, p := range podList.Items {
		if p.GenerateName == podName {
			return &podList.Items[i], nil
		}
	}
	return nil, errors.Errorf("no pods match %s/%s", ns, podName)
}
