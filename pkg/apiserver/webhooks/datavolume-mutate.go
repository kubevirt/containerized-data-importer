/*
 * This file is part of the CDI project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2019 Red Hat, Inc.
 *
 */

package webhooks

import (
	"context"
	"encoding/json"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	admissionv1 "k8s.io/api/admission/v1"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfield "k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/clone"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/token"
)

type dataVolumeMutatingWebhook struct {
	k8sClient      kubernetes.Interface
	cdiClient      cdiclient.Interface
	tokenGenerator token.Generator
	proxy          clone.SubjectAccessReviewsProxy
}

type sarProxy struct {
	client kubernetes.Interface
}

type cloneType int

const (
	noClone cloneType = iota
	pvcClone
	snapshotClone
)

type cloneSourceHandler struct {
	cloneType       cloneType
	tokenResource   metav1.GroupVersionResource
	cloneAuthFunc   clone.UserCloneAuthFunc
	sourceName      string
	sourceNamespace string
}

var (
	tokenResourcePvc = metav1.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "persistentvolumeclaims",
	}

	tokenResourceSnapshot = metav1.GroupVersionResource{
		Group:    snapshotv1.GroupName,
		Version:  snapshotv1.SchemeGroupVersion.Version,
		Resource: "volumesnapshots",
	}
)

func (p *sarProxy) Create(sar *authv1.SubjectAccessReview) (*authv1.SubjectAccessReview, error) {
	return p.client.AuthorizationV1().SubjectAccessReviews().Create(context.TODO(), sar, metav1.CreateOptions{})
}

func (wh *dataVolumeMutatingWebhook) Admit(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	var dataVolume, oldDataVolume cdiv1.DataVolume

	klog.V(3).Infof("Got AdmissionReview %+v", ar)

	if err := validateDataVolumeResource(ar); err != nil {
		return toAdmissionResponseError(err)
	}

	if err := json.Unmarshal(ar.Request.Object.Raw, &dataVolume); err != nil {
		return toAdmissionResponseError(err)
	}

	if dataVolume.GetDeletionTimestamp() != nil {
		// No point continuing if DV is flagged for deletion
		return allowedAdmissionResponse()
	}

	cloneSourceHandler, err := newCloneSourceHandler(&dataVolume, wh.cdiClient)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	targetNamespace, targetName := dataVolume.Namespace, dataVolume.Name
	if targetNamespace == "" {
		targetNamespace = ar.Request.Namespace
	}

	if targetName == "" {
		targetName = ar.Request.Name
	}

	modifiedDataVolume := dataVolume.DeepCopy()
	modified := false

	if ar.Request.Operation == admissionv1.Create {
		config, err := wh.cdiClient.CdiV1beta1().CDIConfigs().Get(context.TODO(), common.ConfigName, metav1.GetOptions{})
		if err != nil {
			return toAdmissionResponseError(err)
		}
		if cc.GetDataVolumeTTLSeconds(config) >= 0 {
			if modifiedDataVolume.Annotations == nil {
				modifiedDataVolume.Annotations = make(map[string]string)
			}
			if modifiedDataVolume.Annotations[cc.AnnDeleteAfterCompletion] != "false" {
				modifiedDataVolume.Annotations[cc.AnnDeleteAfterCompletion] = "true"
				modified = true
			}
		}
	}

	if cloneSourceHandler.cloneType == noClone {
		klog.V(3).Infof("DataVolume %s/%s not cloning", targetNamespace, targetName)
		if modified {
			return toPatchResponse(dataVolume, modifiedDataVolume)
		}
		return allowedAdmissionResponse()
	}

	sourceName, sourceNamespace := cloneSourceHandler.sourceName, cloneSourceHandler.sourceNamespace
	if sourceNamespace == "" {
		sourceNamespace = targetNamespace
	}

	_, err = wh.k8sClient.CoreV1().Namespaces().Get(context.TODO(), sourceNamespace, metav1.GetOptions{})
	if err != nil {
		return toAdmissionResponseError(err)
	}

	if ar.Request.Operation == admissionv1.Update {
		if err := json.Unmarshal(ar.Request.OldObject.Raw, &oldDataVolume); err != nil {
			return toAdmissionResponseError(err)
		}

		_, ok := oldDataVolume.Annotations[cc.AnnCloneToken]
		if ok {
			klog.V(3).Infof("DataVolume %s/%s already has clone token", targetNamespace, targetName)
			return allowedAdmissionResponse()
		}
	}

	ok, reason, err := cloneSourceHandler.cloneAuthFunc(wh.proxy, sourceNamespace, sourceName, targetNamespace, ar.Request.UserInfo)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	if !ok {
		causes := []metav1.StatusCause{
			{
				Type:    metav1.CauseTypeFieldValueInvalid,
				Message: reason,
				Field:   k8sfield.NewPath("spec", "source", "PVC", "namespace").String(),
			},
		}
		return toRejectedAdmissionResponse(causes)
	}

	tokenData := &token.Payload{
		Operation: token.OperationClone,
		Name:      sourceName,
		Namespace: sourceNamespace,
		Resource:  cloneSourceHandler.tokenResource,
		Params: map[string]string{
			"targetNamespace": targetNamespace,
			"targetName":      targetName,
		},
	}

	token, err := wh.tokenGenerator.Generate(tokenData)
	if err != nil {
		return toAdmissionResponseError(err)
	}

	if modifiedDataVolume.Annotations == nil {
		modifiedDataVolume.Annotations = make(map[string]string)
	}
	modifiedDataVolume.Annotations[cc.AnnCloneToken] = token

	klog.V(3).Infof("Sending patch response...")

	return toPatchResponse(dataVolume, modifiedDataVolume)
}

func newCloneSourceHandler(dataVolume *cdiv1.DataVolume, cdiClient cdiclient.Interface) (*cloneSourceHandler, error) {
	var pvcSource *cdiv1.DataVolumeSourcePVC
	var snapshotSource *cdiv1.DataVolumeSourceSnapshot

	if dataVolume.Spec.Source != nil {
		if dataVolume.Spec.Source.PVC != nil {
			pvcSource = dataVolume.Spec.Source.PVC
		} else if dataVolume.Spec.Source.Snapshot != nil {
			snapshotSource = dataVolume.Spec.Source.Snapshot
		}
	} else if dataVolume.Spec.SourceRef != nil && dataVolume.Spec.SourceRef.Kind == cdiv1.DataVolumeDataSource {
		ns := dataVolume.Namespace
		if dataVolume.Spec.SourceRef.Namespace != nil && *dataVolume.Spec.SourceRef.Namespace != "" {
			ns = *dataVolume.Spec.SourceRef.Namespace
		}
		dataSource, err := cdiClient.CdiV1beta1().DataSources(ns).Get(context.TODO(), dataVolume.Spec.SourceRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if dataSource.Spec.Source.PVC != nil {
			pvcSource = dataSource.Spec.Source.PVC
		} else if dataSource.Spec.Source.Snapshot != nil {
			snapshotSource = dataSource.Spec.Source.Snapshot
		}
	}

	switch {
	case pvcSource != nil:
		return &cloneSourceHandler{
			cloneType:       pvcClone,
			tokenResource:   tokenResourcePvc,
			cloneAuthFunc:   clone.CanUserClonePVC,
			sourceName:      pvcSource.Name,
			sourceNamespace: pvcSource.Namespace,
		}, nil
	case snapshotSource != nil:
		return &cloneSourceHandler{
			cloneType:       snapshotClone,
			tokenResource:   tokenResourceSnapshot,
			cloneAuthFunc:   clone.CanUserCloneSnapshot,
			sourceName:      snapshotSource.Name,
			sourceNamespace: snapshotSource.Namespace,
		}, nil
	default:
		return &cloneSourceHandler{
			cloneType: noClone,
		}, nil
	}
}
