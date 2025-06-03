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

type ReplicatedStorageClass struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ReplicatedStorageClassSpec   `json:"spec"`
	Status            ReplicatedStorageClassStatus `json:"status,omitempty"`
}

// ReplicatedStorageClassList contains a list of empty block device
type ReplicatedStorageClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ReplicatedStorageClass `json:"items"`
}

type ReplicatedStorageClassSpec struct {
	StoragePool   string   `json:"storagePool"`
	ReclaimPolicy string   `json:"reclaimPolicy"`
	Replication   string   `json:"replication"`
	VolumeAccess  string   `json:"volumeAccess"`
	Topology      string   `json:"topology"`
	Zones         []string `json:"zones"`
}

type ReplicatedStorageClassStatus struct {
	Phase  string `json:"phase,omitempty"`
	Reason string `json:"reason,omitempty"`
}
