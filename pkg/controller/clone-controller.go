package controller

import (
	"fmt"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	. "kubevirt.io/containerized-data-importer/pkg/common"
	"time"
)

const (
	//AnnCloneRequest sets our expected annotation for a CloneRequest
	AnnCloneRequest = "k8s.io/CloneRequest"
	AnnCloneOf      = "k8s.io/CloneOf"
	// cloner pods annotations
	AnnClonePodPhase      = "cdi.kubevirt.io/storage.clone.pod.phase"
	CloneUniqueID         = "cdi.kubevirt.io/storage.clone.cloneUniqeId"
	AnnTargetPodNamespace = "cdi.kubevirt.io/storage.clone.targetPod.namespace"
)

// CloneController represents the CDI Clone Controller
type CloneController struct {
	Controller
}

// NewCloneController sets up a Clone Controller, and returns a pointer to
// to the newly created Controller
func NewCloneController(client kubernetes.Interface,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	podInformer coreinformers.PodInformer,
	image string,
	pullPolicy string,
	verbose string) *CloneController {
	c := &CloneController{
		Controller: *NewController(client, pvcInformer, podInformer, image, pullPolicy, verbose),
	}
	return c
}

func (cc *CloneController) findClonePodsFromCache(pvc *v1.PersistentVolumeClaim) (*v1.Pod, *v1.Pod, error) {
	var sourcePod, targetPod *v1.Pod
	annCloneRequest := pvc.GetAnnotations()[AnnCloneRequest]
	if annCloneRequest != "" {
		sourcePvcNamespace, _ := ParseSourcePvcAnnotation(annCloneRequest, "/")
		if sourcePvcNamespace == "" {
			return nil, nil, errors.Errorf("Bad CloneRequest Annotation")
		}
		//find the source pod
		selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: map[string]string{CloneUniqueID: pvc.Name + "-source-pod"}})
		if err != nil {
			return nil, nil, err
		}
		podList, err := cc.podLister.Pods(sourcePvcNamespace).List(selector)
		if err != nil {
			return nil, nil, err
		}
		if len(podList) == 0 {
			return nil, nil, nil
		} else if len(podList) > 1 {
			return nil, nil, errors.Errorf("multiple source pods found for clone PVC %s/%s", pvc.Namespace, pvc.Name)
		}
		sourcePod = podList[0]
		//find target pod
		selector, err = metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: map[string]string{CloneUniqueID: pvc.Name + "-target-pod"}})
		if err != nil {
			return nil, nil, err
		}
		podList, err = cc.podLister.Pods(pvc.Namespace).List(selector)
		if err != nil {
			return nil, nil, err
		}
		if len(podList) == 0 {
			return nil, nil, nil
		} else if len(podList) > 1 {
			return nil, nil, errors.Errorf("multiple target pods found for clone PVC %s/%s", pvc.Namespace, pvc.Name)
		}
		targetPod = podList[0]
	}
	return sourcePod, targetPod, nil
}

// Create the cloning source and target pods based the pvc. The pvc is checked (again) to ensure that we are not already
// processing this pvc, which would result in multiple pods for the same pvc.
func (cc *CloneController) processPvcItem(pvc *v1.PersistentVolumeClaim) error {
	anno := map[string]string{}

	// find cloning source and target Pods
	sourcePod, targetPod, err := cc.findClonePodsFromCache(pvc)
	if err != nil {
		return err
	}
	pvcKey, err := cache.MetaNamespaceKeyFunc(pvc)
	if err != nil {
		return err
	}

	// Pods must be controlled by this PVC
	if sourcePod != nil && !metav1.IsControlledBy(sourcePod, pvc) {
		return errors.Errorf("found pod %s/%s not owned by pvc %s/%s", sourcePod.Namespace, sourcePod.Name, pvc.Namespace, pvc.Name)
	}
	if targetPod != nil && !metav1.IsControlledBy(sourcePod, pvc) {
		return errors.Errorf("found pod %s/%s not owned by pvc %s/%s", targetPod.Namespace, targetPod.Name, pvc.Namespace, pvc.Name)
	}

	// expectations prevent us from creating multiple pods. An expectation forces
	// us to observe a pod's creation in the cache.
	needsSync := cc.podExpectations.SatisfiedExpectations(pvcKey)

	// make sure not to reprocess a PVC that has already completed successfully,
	// even if the pod no longer exists
	phase, exists := pvc.ObjectMeta.Annotations[AnnClonePodPhase]
	if exists && (phase == string(v1.PodSucceeded)) {
		needsSync = false
	}

	if needsSync && (sourcePod == nil || targetPod == nil) {
		err := c.initializeExpectations(pvcKey)
		if err != nil {
			return err
		}
		//create random string to be used for pod labeling and hostpath name
		if sourcePod == nil {
			cr, err := getCloneRequestPVC(pvc)
			if err != nil {
				return err
			}
			// all checks passed, let's create the cloner pods!
			cc.expectPodCreate(pvcKey)
			//create the source pod
			sourcePod, err = CreateCloneSourcePod(cc.clientset, cc.image, cc.verbose, cc.pullPolicy, cr, pvc)
			if err != nil {
				cc.observePodCreate(pvcKey)
				return err
			}
		}
		if targetPod == nil {
			cc.expectPodCreate(pvcKey)
			//create the target pod
			targetPod, err = CreateCloneTargetPod(cc.clientset, cc.image, cc.verbose, cc.pullPolicy, pvc, sourcePod.ObjectMeta.Namespace)
			if err != nil {
				cc.observePodCreate(pvcKey)
				return err
			}
		}
		return nil
	}

	// update pvc with cloner pod name and optional cdi label
	//we update the target PVC according to the target pod. Only the target pods indicates the real status of the cloning.
	anno[AnnClonePodPhase] = string(targetPod.Status.Phase)
	//add the following annotation only if the pod pahse is succeeded, meaning job is completed
	if phase == string(v1.PodSucceeded) {
		anno[AnnCloneOf] = "true"
	}
	var lab map[string]string
	if !checkIfLabelExists(pvc, common.CDI_LABEL_KEY, common.CDI_LABEL_VALUE) {
		lab = map[string]string{common.CDI_LABEL_KEY: common.CDI_LABEL_VALUE}
	}
	pvc, err = updatePVC(cc.clientset, pvc, anno, lab)
	if err != nil {
		return errors.WithMessage(err, "could not update pvc %q annotation and/or label")
	}
	return nil
}

// Select only pvcs with the 'CloneRequest' annotation and that are not being processed.
// We forget the key unless `processPvcItem` returns an error in which case the key can be
// retried.
func (cc *CloneController) ProcessNextPvcItem() bool {
	key, shutdown := cc.queue.Get()
	if shutdown {
		return false
	}
	defer cc.queue.Done(key)

	err := cc.syncPvc(key.(string))
	if err != nil { // processPvcItem errors may not have been logged so log here
		glog.Errorf("error processing pvc %q: %v", key, err)
		return true
	}
	return cc.forgetKey(key, fmt.Sprintf("ProcessNextPvcItem: processing pvc %q completed", key))
}

func (cc *CloneController) syncPvc(key string) error {
	pvc, err := cc.pvcFromKey(key)
	if err != nil || pvc == nil {
		return err
	}

	//check if AnnoCloneRequest annotation exists
	if !checkPVC(pvc, AnnCloneRequest) {
		return nil
	}
	//checking for CloneOf annotation indicating that the clone was already taken care of by the provisioner (smart clone).
	if metav1.HasAnnotation(pvc.ObjectMeta, AnnCloneOf) {
		glog.V(Vadmin).Infof("pvc annotation %q exists indicating cloning completed, skipping pvc\n", AnnCloneOf)
		return nil
	}
	glog.V(Vdebug).Infof("ProcessNextPvcItem: next pvc to process: %s\n", key)
	return cc.processPvcItem(pvc)
}

func (cc *CloneController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer func() {
		cc.queue.ShutDown()
	}()
	cc.Controller.Run(threadiness, stopCh)
	for i := 0; i < threadiness; i++ {
		go wait.Until(cc.runPVCWorkers, time.Second, stopCh)
	}
	<-stopCh
	return nil
}

func (cc *CloneController) runPVCWorkers() {
	for cc.ProcessNextPvcItem() {
	}
}
