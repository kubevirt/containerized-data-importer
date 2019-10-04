package controller

import (
	"crypto/rand"
	"crypto/rsa"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/cert"

	"kubevirt.io/containerized-data-importer/pkg/util/cert/triple"
)

type CloneFixture struct {
	ControllerFixture
}

var (
	apiServerKey     *rsa.PrivateKey
	apiServerKeyOnce sync.Once

	testUploadServerCASecret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cdi-upload-server-ca-key",
			Namespace: "cdi",
		},
		Data: map[string][]byte{
			"tls.key": []byte("privatekey"),
			"tls.crt": []byte("cert"),
		},
	}

	testUploadServerClientCASecret     *corev1.Secret
	testUploadServerClientCASecretOnce sync.Once
)

func testCreateClientKeyAndCert(ca *triple.KeyPair, commonName string, organizations []string) ([]byte, []byte, error) {
	return []byte("foo"), []byte("bar"), nil
}

func getAPIServerKey() *rsa.PrivateKey {
	apiServerKeyOnce.Do(func() {
		apiServerKey, _ = rsa.GenerateKey(rand.Reader, 2048)
	})
	return apiServerKey
}

func getUploadServerClientCASecret() *corev1.Secret {
	testUploadServerClientCASecretOnce.Do(func() {
		keypair, _ := triple.NewCA("BAZ")
		kb := cert.EncodePrivateKeyPEM(keypair.Key)
		cb := cert.EncodeCertPEM(keypair.Cert)
		testUploadServerClientCASecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cdi-upload-server-client-ca-key",
				Namespace: "cdi",
			},
			Data: map[string][]byte{
				"tls.key": kb,
				"tls.crt": cb,
			},
		}
	})
	return testUploadServerClientCASecret
}

func newCloneFixture(t *testing.T) *CloneFixture {
	f := &CloneFixture{
		ControllerFixture: *newControllerFixture(t),
	}
	return f
}

func (f *CloneFixture) newCloneController() *CloneController {
	v := newCloneTokenValidator(&getAPIServerKey().PublicKey)
	return &CloneController{
		Controller:     *f.newController("test/mycloneimage", "Always", "5"),
		recorder:       &record.FakeRecorder{},
		tokenValidator: v,
	}
}

func (f *CloneFixture) run(pvcName string) {
	f.runController(pvcName, true, false, false)
}

func (f *CloneFixture) runWithExpectation(pvcName string) {
	f.runController(pvcName, true, false, true)
}

func (f *CloneFixture) runExpectError(pvcName string) {
	f.runController(pvcName, true, true, false)
}

func (f *CloneFixture) runController(pvcName string,
	startInformers bool,
	expectError bool,
	withCreateExpectation bool) {
	c := f.newCloneController()
	if startInformers {
		stopCh := make(chan struct{})
		defer close(stopCh)
		go c.pvcInformer.Run(stopCh)
		go c.podInformer.Run(stopCh)
		cache.WaitForCacheSync(stopCh, c.pvcInformer.HasSynced)
		cache.WaitForCacheSync(stopCh, c.pvcInformer.HasSynced)
	}

	if withCreateExpectation {
		c.raisePodCreate(pvcName)
	}
	err := c.syncPvc(pvcName)
	if !expectError && err != nil {
		f.t.Errorf("error syncing pvc: %s: %v", pvcName, err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing pvc, got nil")
	}

	k8sActions := filterActions(f.kubeclient.Actions())
	for i, action := range k8sActions {
		if len(f.kubeactions) < i+1 {
			f.t.Errorf("%d unexpected actions: %+v", len(k8sActions)-len(f.kubeactions), k8sActions[i:])
			break
		}

		expectedAction := f.kubeactions[i]
		checkAction(expectedAction, action, f.t)
	}

	if len(f.kubeactions) > len(k8sActions) {
		f.t.Errorf("%d additional expected actions:%+v", len(f.kubeactions)-len(k8sActions), f.kubeactions[len(k8sActions):])
	}
}

func TestWaitsTargetRunning(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createClonePvc("source-ns", "golden-pvc", "target-ns", "target-pvc", nil, nil)
	pvc.Annotations[AnnPodReady] = "false"

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.run(getPvcKey(pvc, t))
}

func TestWaitsTargetRunningNoAnnotation(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createClonePvc("source-ns", "golden-pvc", "target-ns", "target-pvc", nil, nil)

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.run(getPvcKey(pvc, t))
}

func TestCreatesSourcePod(t *testing.T) {
	createClientKeyAndCertFunc = testCreateClientKeyAndCert
	defer func() {
		createClientKeyAndCertFunc = createClientKeyAndCert
	}()
	f := newCloneFixture(t)
	sourcePvc := createPvc("golden-pvc", "source-ns", nil, nil)
	pvc := createClonePvc("source-ns", "golden-pvc", "target-ns", "target-pvc", nil, nil)
	pvc.Annotations[AnnPodReady] = "true"

	f.pvcLister = append(f.pvcLister, sourcePvc, pvc)
	f.kubeobjects = append(f.kubeobjects, testUploadServerCASecret, getUploadServerClientCASecret(), sourcePvc, pvc)

	id := string(pvc.GetUID())
	expSourcePod := createSourcePod(pvc, id)
	pvcUpdate := pvc.DeepCopy()
	pvcUpdate.Finalizers = []string{cloneSourcePodFinalizer}
	f.expectUpdatePvcAction(pvcUpdate)
	f.expectSecretGetAction(testUploadServerCASecret)
	f.expectSecretGetAction(getUploadServerClientCASecret())
	f.expectCreatePodAction(expSourcePod)

	f.run(getPvcKey(pvc, t))
}

func TestAddsCloneOfAnnotation(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createClonePvc("source-ns", "golden-pvc", "target-ns", "target-pvc", nil, nil)
	pvc.Annotations[AnnPodReady] = "true"
	pvc.Annotations[AnnPodPhase] = string(corev1.PodSucceeded)
	id := string(pvc.GetUID())
	pod := createSourcePod(pvc, id)
	pod.Namespace = "source-ns"

	f.pvcLister = append(f.pvcLister, pvc)
	f.podLister = append(f.podLister, pod)
	f.kubeobjects = append(f.kubeobjects, pvc, pod)

	updatedPVC := pvc.DeepCopy()
	updatedPVC.Annotations[AnnCloneOf] = "true"
	f.expectUpdatePvcAction(updatedPVC)
	f.run(getPvcKey(pvc, t))
}

func TestDeletesSourcePodAndFinalizer(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createClonePvc("source-ns", "golden-pvc", "target-ns", "target-pvc", nil, nil)
	pvc.Annotations[AnnCloneOf] = "true"
	pvc.Finalizers = []string{cloneSourcePodFinalizer}
	id := string(pvc.GetUID())
	pod := createSourcePod(pvc, id)
	pod.Name = pod.GenerateName + "random"
	pod.Namespace = "source-ns"

	f.pvcLister = append(f.pvcLister, pvc)
	f.podLister = append(f.podLister, pod)
	f.kubeobjects = append(f.kubeobjects, pvc, pod)

	pvcUpdate := pvc.DeepCopy()
	pvcUpdate.Finalizers = nil

	f.expectDeletePodAction(pod)
	f.expectUpdatePvcAction(pvcUpdate)
	f.run(getPvcKey(pvc, t))
}

func TestSourceDoesNotExist(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createClonePvc("source-ns", "golden-pvc", "target-ns", "target-pvc", nil, nil)
	pvc.Annotations[AnnPodReady] = "true"

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.runExpectError(getPvcKey(pvc, t))
}

func TestExpectationsNotMet(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createClonePvc("source-ns", "golden-pvc", "target-ns", "target-pvc", nil, nil)
	pvc.Annotations[AnnPodReady] = "true"

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.runWithExpectation(getPvcKey(pvc, t))
}

// Verifies that one cannot clone a fs pvc to a block pvc
func TestCannotCloneFSToBlockPvc(t *testing.T) {
	f := newCloneFixture(t)
	sourcePvc := createPvc("golden-pvc", "source-ns", nil, nil)
	pvc := createCloneBlockPvc("source-ns", "golden-pvc", "target-ns", "target-pvc", nil, nil)
	pvc.Annotations[AnnPodReady] = "true"

	f.pvcLister = append(f.pvcLister, sourcePvc, pvc)
	f.kubeobjects = append(f.kubeobjects, sourcePvc, pvc)
	f.runExpectError(getPvcKey(pvc, t))
}

// Verifies that one cannot clone a fs pvc to a block pvc
func TestCannotCloneBlockToFSPvc(t *testing.T) {
	f := newCloneFixture(t)
	sourcePvc := createBlockPvc("golden-pvc", "source-ns", nil, nil)
	pvc := createClonePvc("source-ns", "golden-pvc", "target-ns", "target-pvc", nil, nil)
	pvc.Annotations[AnnPodReady] = "true"

	f.pvcLister = append(f.pvcLister, sourcePvc)
	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, sourcePvc)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.runExpectError(getPvcKey(pvc, t))
}

// Verifies that one cannot clone a fs pvc to a block pvc
func TestCannotCloneIfTargetIsSmaller(t *testing.T) {
	f := newCloneFixture(t)
	sourcePvc := createPvc("golden-pvc", "source-ns", nil, nil)
	pvc := createClonePvcWithSize("source-ns", "golden-pvc", "target-ns", "target-pvc", nil, nil, "500M")
	pvc.Annotations[AnnPodReady] = "true"

	f.pvcLister = append(f.pvcLister, sourcePvc)
	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, sourcePvc)
	f.kubeobjects = append(f.kubeobjects, pvc)
	f.runExpectError(getPvcKey(pvc, t))
}

// verifies no work is done on pvcs without our annotations
func TestCloneIgnorePVC(t *testing.T) {
	f := newCloneFixture(t)
	pvc := createPvc("target-pvc", "target-ns", nil, nil)

	f.pvcLister = append(f.pvcLister, pvc)
	f.kubeobjects = append(f.kubeobjects, pvc)

	f.run(getPvcKey(pvc, t))
}
