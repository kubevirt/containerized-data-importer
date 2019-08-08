/*
Copyright 2018 The CDI Authors.

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

package controller

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/appscode/jsonpatch"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func mergeLabelsAndAnnotations(src, dest metav1.Object) {
	// allow users to add labels but not change ours
	for k, v := range src.GetLabels() {
		if dest.GetLabels() == nil {
			dest.SetLabels(map[string]string{})
		}

		dest.GetLabels()[k] = v
	}

	// same for annotations
	for k, v := range src.GetAnnotations() {
		if dest.GetAnnotations() == nil {
			dest.SetAnnotations(map[string]string{})
		}

		dest.GetAnnotations()[k] = v
	}
}

func mergeObject(desiredObj, currentObj runtime.Object) (runtime.Object, error) {
	// copy labels/annotations that may have been merged above
	desiredRuntimeObjCopy := desiredObj.DeepCopyObject()
	currentRuntimeObjCopy := currentObj.DeepCopyObject()

	desiredMetaObjCopy := desiredRuntimeObjCopy.(metav1.Object)
	currentMetaObjCopy := currentRuntimeObjCopy.(metav1.Object)

	desiredMetaObjCopy.SetLabels(currentMetaObjCopy.GetLabels())
	desiredMetaObjCopy.SetAnnotations(currentMetaObjCopy.GetAnnotations())

	// for some reason, null creationTimestamp gets encoded
	desiredMetaObjCopy.SetCreationTimestamp(currentMetaObjCopy.GetCreationTimestamp())

	desiredBytes, err := json.Marshal(desiredRuntimeObjCopy)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(desiredBytes, currentRuntimeObjCopy); err != nil {
		return nil, err
	}

	return currentRuntimeObjCopy, nil
}

func deployClusterResources() bool {
	return strings.ToLower(os.Getenv("DEPLOY_CLUSTER_RESOURCES")) != "false"
}

func logJSONDiff(logger logr.Logger, objA, objB interface{}) {
	aBytes, _ := json.Marshal(objA)
	bBytes, _ := json.Marshal(objB)
	patches, _ := jsonpatch.CreatePatch(aBytes, bBytes)
	pBytes, _ := json.Marshal(patches)
	logger.Info("DIFF", "obj", objA, "patch", string(pBytes))
}

func checkDeploymentReady(deployment *appsv1.Deployment) bool {
	desiredReplicas := deployment.Spec.Replicas
	if desiredReplicas == nil {
		desiredReplicas = &[]int32{1}[0]
	}

	if *desiredReplicas != deployment.Status.Replicas ||
		deployment.Status.Replicas != deployment.Status.ReadyReplicas {
		return false
	}

	return true
}
