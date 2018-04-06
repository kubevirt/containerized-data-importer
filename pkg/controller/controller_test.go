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
		// anno may be updated in It block
		pvcObj = &v1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:        pvcName,
				Annotations: map[string]string{AnnEndpoint: ""},
			},
		}
	})

	AfterEach(func() {
		close(stop)
	})

	tests := []testT{
		{
			descr:         "pvc with endpoint: controller creates importer pod",
			annEndpoint:   "http://www.google.com",
			expectPodName: "importer-" + pvcName,
			expectError:   false,
		},
		{
			descr:         "pvc with blank endpoint: controller does not create importer pod",
			annEndpoint:   "",
			expectPodName: "",
			expectError:   true,
		},
	}

	for _, test := range tests {
		ep := test.annEndpoint
		exptPod := test.expectPodName
		exptErr := test.expectError
		It(test.descr, func() {
			By(fmt.Sprintf("setting the pvc's %q anno to %q", AnnEndpoint, ep))
			pvcObj.Annotations[AnnEndpoint] = ep
			By("invoking the controller")
			setUpInformer(pvcObj, opAdd, pvcName)
			controller.ProcessNextItem()
			By("checking if importer pod is present")
			pod := getImporterPod(fakeClient, exptPod)
			if exptErr {
				Expect(pod).To(BeNil(), "pod should not exist")
			} else {
				Expect(pod).NotTo(BeNil(), fmt.Sprintf("pod %q was not found", exptPod))
				Expect(pod.GenerateName).To(HavePrefix(exptPod))
			}
		})
	}
})

// getImporterPod gets the first pod with a generated name equal to the passed-in name.
// Nil is returned if no match is found.
func getImporterPod(fc *fake.Clientset, podName string) *v1.Pod {
	podList, err := fc.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil || len(podList.Items) == 0 {
		return nil
	}
	for i, p := range podList.Items {
		if p.GenerateName == podName {
			return &podList.Items[i]
		}
	}
	return nil
}
