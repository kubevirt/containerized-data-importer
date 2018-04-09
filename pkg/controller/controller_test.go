package controller_test

import (
	"fmt"

	. "github.com/kubevirt/containerized-data-importer/pkg/controller"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	k8stesting "k8s.io/client-go/tools/cache/testing"
	"k8s.io/client-go/util/workqueue"
)

type operation int

const (
	opAdd operation = iota
	opUpdate
	opDelete
)

var _ = Describe("Controller", func() {
	const pvcName = "test-pvc"
	var (
		controller *Controller
		fakeClient *fake.Clientset
		pvcObj     *v1.PersistentVolumeClaim
		stop       chan struct{}
	)
	type testT struct {
		descr         string
		ns            string
		annEndpoint   string
		expectPodName string
		expectError   bool
	}

	setUpInformer := func(obj *v1.PersistentVolumeClaim, op operation, pvcName string) {
		stop = make(chan struct{})
		fakeClient = fake.NewSimpleClientset()
		importerTag := "latest"
		objSource := k8stesting.NewFakeControllerSource()
		pvcInformer := cache.NewSharedIndexInformer(objSource, obj, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
		pvcListWatcher := k8stesting.NewFakeControllerSource()
		controller = NewController(fakeClient, queue, pvcInformer, pvcListWatcher, importerTag)
		if op == opAdd {
			pvcListWatcher.Add(obj)
			objSource.Add(obj)
			queue.Add(pvcName)
		}
		go pvcInformer.Run(stop)
		Expect(cache.WaitForCacheSync(stop, pvcInformer.HasSynced)).To(BeTrue())
	}

	BeforeEach(func() {
		// anno and namespace may be updated in It block
		pvcObj = &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        pvcName,
				Namespace:   "",
				Annotations: map[string]string{AnnEndpoint: ""},
			},
		}
	})

	AfterEach(func() {
		close(stop)
	})

	tests := []testT{
		{
			descr:         "pvc, endpoint, blank ns: controller creates importer pod",
			ns:            "", // seems to be default for unit tests
			annEndpoint:   "http://www.google.com",
			expectPodName: "importer-" + pvcName,
			expectError:   false,
		},
		{
			descr:         "pvc, endpoint, non-blank ns: controller creates importer pod",
			ns:            "ns-a",
			annEndpoint:   "http://www.google.com",
			expectPodName: "importer-" + pvcName,
			expectError:   false,
		},
		{
			descr:         "pvc, blank endpoint: controller does not create importer pod",
			ns:            "",
			annEndpoint:   "",
			expectPodName: "",
			expectError:   true,
		},
	}

	for _, test := range tests {
		ep := test.annEndpoint
		ns := test.ns
		exptPod := test.expectPodName
		exptErr := test.expectError
		It(test.descr, func() {
			By(fmt.Sprintf("setting the pvc's endpt anno=%q and ns=%q", ep, ns))
			pvcObj.Annotations[AnnEndpoint] = ep
			pvcObj.Namespace = ns
			By("invoking the controller")
			setUpInformer(pvcObj, opAdd, pvcName)
			controller.ProcessNextItem()
			By("checking if importer pod is present")
			pod, err := getImporterPod(fakeClient, ns, exptPod)
			if exptErr {
				Expect(err).ToNot(BeNil(), fmt.Sprintf("importer pod %s... should not exist\n", exptPod))
			} else {
				Expect(err).To(BeNil(), fmt.Sprintf("importer pod: %v\n", err))
				Expect(pod).ToNot(BeNil(), fmt.Sprintf("importer pod %q missing", exptPod))
				Expect(pod.GenerateName).To(HavePrefix(exptPod))
			}
		})
	}
})

// getImporterPod gets the first pod with a generated name equal to the passed-in name.
// Nil is returned if no match is found.
func getImporterPod(fc *fake.Clientset, ns, podName string) (*v1.Pod, error) {
	podList, err := fc.CoreV1().Pods(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("getImporterPod: %v\n", err)
	}
	if len(podList.Items) == 0 {
		return nil, fmt.Errorf("getImporterPod: no pods found in namespace %q\n", ns)
	}
	for i, p := range podList.Items {
		if p.GenerateName == podName {
			return &podList.Items[i], nil
		}
	}
	return nil, fmt.Errorf("getImporterPod: no pods match %s/%s\n", ns, podName)
}
