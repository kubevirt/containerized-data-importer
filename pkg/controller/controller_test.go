// +build unit_test

package controller_test

import (
	"fmt"

	"github.com/kubevirt/containerized-data-importer/pkg/common"
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
	var (
		controller *Controller
		fakeClient *fake.Clientset
		stop       chan struct{}
	)
	type testT struct {
		descr       string
		ns          string
		name        string // name of test pvc
		qops        operation
		annotations map[string]string
		expectError bool
	}

	setUpInformer := func(pvc *v1.PersistentVolumeClaim, op operation) {
		// build queue value of ns + "/" + pvc name if exists
		ns := pvc.Namespace
		name := pvc.Name
		queueKey := name
		if len(ns) > 0 {
			queueKey = fmt.Sprintf("%s/%s", ns, name)
		}

		stop = make(chan struct{})
		fakeClient = fake.NewSimpleClientset()
		importerTag := "latest"
		objSource := k8stesting.NewFakeControllerSource()
		pvcInformer := cache.NewSharedIndexInformer(objSource, pvc, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
		pvcListWatcher := k8stesting.NewFakeControllerSource()
		var err error
		controller, err = NewController(fakeClient, common.CDI_SELECTOR_LABEL,importerTag)
		if error
		if op == opAdd || op == opUpdate{
			pvcListWatcher.Add(pvc)
			objSource.Add(pvc)
			queue.Add(queueKey)
		}
		go pvcInformer.Run(stop)
		Expect(cache.WaitForCacheSync(stop, pvcInformer.HasSynced)).To(BeTrue())
	}

	AfterEach(func() {
		close(stop)
	})

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
		exptPod := fmt.Sprintf("importer-%s", pvcName)
		exptErr := test.expectError

		It(test.descr, func() {
			By(fmt.Sprintf("creating in-mem pvc %q with endpt anno=%q", fullname, annotations))
			pvcObj := createInMemPVC(ns, pvcName, annotations)
			By("invoking the controller")
			setUpInformer(pvcObj, ops)
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
