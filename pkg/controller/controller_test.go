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
	opAdd    operation = iota
	opUpdate
	opDelete
)

var _ = Describe("Controller", func() {
	var stop chan struct{}
	var controller *Controller
	var fakeClient *fake.Clientset
	type testT struct {
		descr         string
		pvcName       string
		annEndpoint   string
		op            operation
		expectPodName string
		errMessage    string
	}
	setUpInformer := func(obj *v1.PersistentVolumeClaim, op operation, pvcName string) {
		stop = make(chan struct{})
		fakeClient = fake.NewSimpleClientset()
		importerTag := "test"
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

	Context("Test Contoller create pods will success if", func() {
		tests := []testT{
			{
				descr:         "it has an annEndpoint",
				pvcName:       "test",
				annEndpoint:   "http://www.google.com",
				op:            opAdd,
				expectPodName: "importer-test",
			},
		}

		for _, test := range tests {
			BeforeEach(func() {
				pvcObj := &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:        test.pvcName,
						Annotations: map[string]string{AnnEndpoint: test.annEndpoint},
					},
				}
				setUpInformer(pvcObj, test.op, test.pvcName)
			})

			AfterEach(func() {
				close(stop)
			})

			It(test.descr, func() {
				Expect(controller.ProcessNextItem()).To(BeTrue())
				pod, err := getTestPod(fakeClient, test.expectPodName)
				Expect(err).NotTo(HaveOccurred())
				Expect(pod).NotTo(BeNil(), fmt.Sprintf("Expected Pod %q was not found.", test.expectPodName))
				Expect(pod.GenerateName).To(HavePrefix(test.expectPodName))
			})
		}
	})

	Context("Test Contoller create pods will failed if", func() {
		tests := []testT{
			{
				descr:         "it does not have an annEndpoint",
				pvcName:       "test",
				annEndpoint:   "",
				op:            opAdd,
				expectPodName: "",
				errMessage:    "pods \"\" not found",
			},
		}
		for _, test := range tests {
			BeforeEach(func() {
				pvcObj := &v1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:        test.pvcName,
						Annotations: map[string]string{AnnEndpoint: test.annEndpoint},
					},
				}
				setUpInformer(pvcObj, test.op, test.pvcName)
			})

			AfterEach(func() {
				close(stop)
			})

			It(test.descr, func() {
				Expect(controller.ProcessNextItem()).To(BeTrue())
				_, err := fakeClient.CoreV1().Pods("").Get(test.expectPodName, metav1.GetOptions{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(test.errMessage))
			})
		}
	})
})

// getTestPod is used to handle pods with generated name by comparing the passed in pod name to a list of pods
// If a match is found, the pod is returned.
// If no match is found, a nil pointer is returned.  This should be checked by the caller.
func getTestPod(fc *fake.Clientset, podName string) (*v1.Pod, error) {
	podList, err := fc.CoreV1().Pods("").List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var pod *v1.Pod
	for i, p := range podList.Items {
		if p.GenerateName == podName {
			pod = &podList.Items[i]
		}
	}
	return pod, nil
}
