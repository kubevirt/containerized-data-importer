package controller

import (
	"crypto/rsa"
	"fmt"
	clientset "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"strconv"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/generator"
)

const (
	cloneControllerAgentName = "clone-controller"

	//AnnCloneRequest sets our expected annotation for a CloneRequest
	AnnCloneRequest = "k8s.io/CloneRequest"
	//AnnCloneOf is used to indicate that cloning was complete
	AnnCloneOf = "k8s.io/CloneOf"
	// AnnCloneToken is the annotation containing the clone token
	AnnCloneToken = "cdi.kubevirt.io/storage.clone.token"

	//CloneUniqueID is used as a special label to be used when we search for the pod
	CloneUniqueID = "cdi.kubevirt.io/storage.clone.cloneUniqeId"

	// ErrIncompatiblePVC provides a const to indicate a clone is not possible due to an incompatible PVC
	ErrIncompatiblePVC = "ErrIncompatiblePVC"

	// APIServerPublicKeyDir is the path to the apiserver public key dir
	APIServerPublicKeyDir = "/var/run/cdi/apiserver/key"

	// APIServerPublicKeyPath is the path to the apiserver public key
	APIServerPublicKeyPath = APIServerPublicKeyDir + "/id_rsa.pub"

	cloneSourcePodFinalizer = "cdi.kubevirt.io/cloneSource"

	cloneTokenLeeway = 10 * time.Second

	uploadClientCertDuration = 365 * 24 * time.Hour
)

// CloneController represents the CDI Clone Controller
type CloneController struct {
	Controller
	recorder            record.EventRecorder
	tokenValidator      token.Validator
	cdiClient           clientset.Interface
	clientCertGenerator generator.CertGenerator
	serverCAFetcher     fetcher.CertBundleFetcher
}

// NewCloneController sets up a Clone Controller, and returns a pointer to
// to the newly created Controller
func NewCloneController(client kubernetes.Interface,
	cdiClientSet clientset.Interface,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	podInformer coreinformers.PodInformer,
	image string,
	pullPolicy string,
	verbose string,
	clientCertGenerator generator.CertGenerator,
	serverCAFetcher fetcher.CertBundleFetcher,
	apiServerKey *rsa.PublicKey) *CloneController {

	// Create event broadcaster
	klog.V(3).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.V(2).Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: client.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: cloneControllerAgentName})

	c := &CloneController{
		Controller:          *NewController(client, pvcInformer, podInformer, image, pullPolicy, verbose),
		recorder:            recorder,
		tokenValidator:      newCloneTokenValidator(apiServerKey),
		cdiClient:           cdiClientSet,
		clientCertGenerator: clientCertGenerator,
		serverCAFetcher:     serverCAFetcher,
	}
	return c
}

func newCloneTokenValidator(key *rsa.PublicKey) token.Validator {
	return token.NewValidator(common.CloneTokenIssuer, key, cloneTokenLeeway)
}

func (cc *CloneController) findCloneSourcePodFromCache(pvc *v1.PersistentVolumeClaim) (*v1.Pod, error) {
	isCloneRequest, sourceNamespace, _ := ParseCloneRequestAnnotation(pvc)
	if !isCloneRequest {
		return nil, nil
	}

	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{
			CloneUniqueID: string(pvc.GetUID()) + "-source-pod",
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "error creating label selector")
	}

	podList, err := cc.podLister.Pods(sourceNamespace).List(selector)
	if err != nil {
		return nil, errors.Wrap(err, "error listing pods")
	}

	if len(podList) > 1 {
		return nil, errors.Errorf("multiple source pods found for clone PVC %s/%s", pvc.Namespace, pvc.Name)
	}

	if len(podList) == 0 {
		return nil, nil
	}

	return podList[0], nil
}

// Create the cloning source and target pods based the pvc. The pvc is checked (again) to ensure that we are not already
// processing this pvc, which would result in multiple pods for the same pvc.
func (cc *CloneController) processPvcItem(pvc *v1.PersistentVolumeClaim) error {
	ready, err := cc.waitTargetPodRunningOrSucceeded(pvc)
	if err != nil {
		return errors.Wrap(err, "error unsuring target upload pod running")
	}

	if !ready {
		klog.V(3).Infof("Upload pod not ready yet for PVC %s/%s", pvc.Namespace, pvc.Name)
		return nil
	}

	pvcKey, err := cache.MetaNamespaceKeyFunc(pvc)
	if err != nil {
		return errors.Wrap(err, "error getting pvcKey")
	}

	// source pod not seen yet
	if !cc.podExpectations.SatisfiedExpectations(pvcKey) {
		klog.V(3).Infof("Waiting on expectations for %s/%s", pvc.Namespace, pvc.Name)
		return nil
	}

	sourcePod, err := cc.findCloneSourcePodFromCache(pvc)
	if err != nil {
		return err
	}

	if sourcePod == nil {
		if err = cc.validateSourceAndTarget(pvc); err != nil {
			return err
		}

		clientName, ok := pvc.Annotations[AnnUploadClientName]
		if !ok {
			return errors.Errorf("PVC %s/%s missing required %s annotation", pvc.Namespace, pvc.Name, AnnUploadClientName)
		}

		pvc, err = addFinalizer(cc.clientset, pvc, cloneSourcePodFinalizer)
		if err != nil {
			return err
		}

		clientCert, clientKey, err := cc.clientCertGenerator.MakeClientCert(clientName, nil, uploadClientCertDuration)
		if err != nil {
			return err
		}

		serverCABundle, err := cc.serverCAFetcher.BundleBytes()
		if err != nil {
			return err
		}

		cc.raisePodCreate(pvcKey)

		args := CloneSourcePodArgs{
			Client:       cc.clientset,
			CDIClient:    cc.cdiClient,
			Image:        cc.image,
			PullPolicy:   cc.pullPolicy,
			ServerCACert: serverCABundle,
			ClientCert:   clientCert,
			ClientKey:    clientKey,
			PVC:          pvc,
		}

		sourcePod, err = CreateCloneSourcePod(args)
		if err != nil {
			cc.observePodCreate(pvcKey)
			return err
		}

		klog.V(3).Infof("Created pod %s/%s", sourcePod.Namespace, sourcePod.Name)
	}

	klog.V(3).Infof("Pod phase for PVC %s/%s is %s", pvc.Namespace, pvc.Name, pvc.Annotations[AnnPodPhase])

	if podSucceededFromPVC(pvc) && pvc.Annotations[AnnCloneOf] != "true" {
		klog.V(1).Infof("Adding CloneOf annotation to PVC %s/%s", pvc.Namespace, pvc.Name)
		pvc.Annotations[AnnCloneOf] = "true"

		_, err := cc.clientset.CoreV1().PersistentVolumeClaims(pvc.Namespace).Update(pvc)
		if err != nil {
			return errors.Wrap(err, "error updating pvc")
		}
	}

	return nil
}

func (cc *CloneController) waitTargetPodRunningOrSucceeded(pvc *v1.PersistentVolumeClaim) (bool, error) {
	rs, ok := pvc.Annotations[AnnPodReady]
	if !ok {
		klog.V(3).Infof("clone target pod for %s/%s not ready", pvc.Namespace, pvc.Name)
		return false, nil
	}

	ready, err := strconv.ParseBool(rs)
	if err != nil {
		return false, errors.Wrapf(err, "error parsing %s annotation", AnnPodReady)
	}

	if !ready {
		klog.V(3).Infof("clone target pod for %s/%s not ready", pvc.Namespace, pvc.Name)
		return podSucceededFromPVC(pvc), nil
	}

	return true, nil
}

func (c *Controller) raisePodCreate(pvcKey string) {
	c.podExpectations.ExpectCreations(pvcKey, 1)
}

// Select only pvcs with the 'CloneRequest' annotation and that are not being processed.
// We forget the key unless `processPvcItem` returns an error in which case the key can be
//ProcessNextPvcItem retried.

//ProcessNextPvcItem ...
func (cc *CloneController) ProcessNextPvcItem() bool {
	key, shutdown := cc.queue.Get()
	if shutdown {
		return false
	}
	defer cc.queue.Done(key)

	err := cc.syncPvc(key.(string))
	if err != nil {
		// Put the item back on the workqueue to handle any transient errors.
		cc.queue.AddRateLimited(key.(string))
		// processPvcItem errors may not have been logged so log here
		klog.Errorf("error processing pvc %q: %v", key, err)
		return true
	}
	return cc.forgetKey(key, fmt.Sprintf("ProcessNextPvcItem: processing pvc %q completed", key))
}

func (cc *CloneController) syncPvc(key string) error {
	pvc, exists, err := cc.pvcFromKey(key)
	if err != nil {
		return errors.Wrap(err, "error getting PVC")
	} else if !exists {
		return nil
	}

	if !checkPVC(pvc, AnnCloneRequest) || pvc.DeletionTimestamp != nil || metav1.HasAnnotation(pvc.ObjectMeta, AnnCloneOf) {
		cc.cleanup(key, pvc)
		return nil
	}

	klog.V(3).Infof("ProcessNextPvcItem: next pvc to process: \"%s/%s\"\n", pvc.Namespace, pvc.Name)
	return cc.processPvcItem(pvc)
}

func (cc *CloneController) cleanup(key string, pvc *v1.PersistentVolumeClaim) error {
	klog.V(3).Infof("Cleaning up for PVC %s/%s", pvc.Namespace, pvc.Name)

	pod, err := cc.findCloneSourcePodFromCache(pvc)
	if err != nil {
		return err
	}

	if pod != nil && pod.DeletionTimestamp == nil {
		if podSucceededFromPVC(pvc) && pod.Status.Phase == v1.PodRunning {
			klog.V(3).Infof("Clone succeeded, waiting for source pod %s/%s to stop running", pod.Namespace, pod.Name)
			return nil
		}

		if err = cc.clientset.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{}); err != nil {
			if !k8serrors.IsNotFound(err) {
				return errors.Wrap(err, "error deleting clone source pod")
			}
		}
	}

	_, err = removeFinalizer(cc.clientset, pvc, cloneSourcePodFinalizer)
	if err != nil {
		return err
	}

	cc.podExpectations.DeleteExpectations(key)

	return nil
}

func (cc *CloneController) validateSourceAndTarget(targetPvc *v1.PersistentVolumeClaim) error {
	sourcePvc, err := getCloneRequestSourcePVC(targetPvc, cc.Controller.pvcLister)
	if err != nil {
		return err
	}

	if err = validateCloneToken(cc.tokenValidator, sourcePvc, targetPvc); err != nil {
		return err
	}

	return ValidateCanCloneSourceAndTargetSpec(&sourcePvc.Spec, &targetPvc.Spec)
}

//Run is being called from cdi-controller (cmd)
func (cc *CloneController) Run(threadiness int, stopCh <-chan struct{}) error {
	cc.Controller.run(threadiness, stopCh, cc.runPVCWorkers)
	return nil
}

func (cc *CloneController) runPVCWorkers() {
	for cc.ProcessNextPvcItem() {
	}
}
