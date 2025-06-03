/*
Copyright 2023 Flant JSC

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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ReplicatedStoragePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ReplicatedStoragePoolSpec   `json:"spec"`
	Status            ReplicatedStoragePoolStatus `json:"status,omitempty"`
}

type ReplicatedStoragePoolSpec struct {
	Type            string                                 `json:"type"`
	LVMVolumeGroups []ReplicatedStoragePoolLVMVolumeGroups `json:"lvmVolumeGroups"`
}

type ReplicatedStoragePoolLVMVolumeGroups struct {
	Name         string `json:"name"`
	ThinPoolName string `json:"thinPoolName"`
}

type ReplicatedStoragePoolStatus struct {
	Phase  string `json:"phase"`
	Reason string `json:"reason"`
}

// ReplicatedStoragePoolList contains a list of ReplicatedStoragePool
type ReplicatedStoragePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ReplicatedStoragePool `json:"items"`
}
