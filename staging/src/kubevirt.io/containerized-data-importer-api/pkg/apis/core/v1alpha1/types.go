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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sdkapi "kubevirt.io/controller-lifecycle-operator-sdk/api"
)

// this has to be here otherwise informer-gen doesn't recognize it
// see https://github.com/kubernetes/code-generator/issues/59
// +genclient:nonNamespaced

// CDI is the CDI Operator CRD
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=cdi;cdis,scope=Cluster
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
type CDI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CDISpec `json:"spec"`
	// +optional
	Status CDIStatus `json:"status"`
}

// CertConfig contains the tunables for TLS certificates
type CertConfig struct {
	// The requested 'duration' (i.e. lifetime) of the Certificate.
	Duration *metav1.Duration `json:"duration,omitempty"`

	// The amount of time before the currently issued certificate's `notAfter`
	// time that we will begin to attempt to renew the certificate.
	RenewBefore *metav1.Duration `json:"renewBefore,omitempty"`
}

// CDICertConfig has the CertConfigs for CDI
type CDICertConfig struct {
	// CA configuration
	// CA certs are kept in the CA bundle as long as they are valid
	CA *CertConfig `json:"ca,omitempty"`

	// Server configuration
	// Certs are rotated and discarded
	Server *CertConfig `json:"server,omitempty"`
}

// CDISpec defines our specification for the CDI installation
type CDISpec struct {
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	// PullPolicy describes a policy for if/when to pull a container image
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty" valid:"required"`
	// +kubebuilder:validation:Enum=RemoveWorkloads;BlockUninstallIfWorkloadsExist
	// CDIUninstallStrategy defines the state to leave CDI on uninstall
	UninstallStrategy *CDIUninstallStrategy `json:"uninstallStrategy,omitempty"`
	// Rules on which nodes CDI infrastructure pods will be scheduled
	Infra sdkapi.NodePlacement `json:"infra,omitempty"`
	// Restrict on which nodes CDI workload pods will be scheduled
	Workloads sdkapi.NodePlacement `json:"workload,omitempty"`
	// Clone strategy override: should we use a host-assisted copy even if snapshots are available?
	// +kubebuilder:validation:Enum="copy";"snapshot"
	CloneStrategyOverride *CDICloneStrategy `json:"cloneStrategyOverride,omitempty"`
	// CDIConfig at CDI level
	Config *CDIConfigSpec `json:"config,omitempty"`
	// certificate configuration
	CertConfig *CDICertConfig `json:"certConfig,omitempty"`
}

// CDICloneStrategy defines the preferred method for performing a CDI clone (override snapshot?)
type CDICloneStrategy string

const (
	// CloneStrategyHostAssisted specifies slower, host-assisted copy
	CloneStrategyHostAssisted = "copy"

	// CloneStrategySnapshot specifies snapshot-based copying
	CloneStrategySnapshot = "snapshot"

	// CloneStrategyCsiClone specifies csi volume clone based cloning
	CloneStrategyCsiClone = "csi-clone"
)

// CDIUninstallStrategy defines the state to leave CDI on uninstall
type CDIUninstallStrategy string

const (
	// CDIUninstallStrategyRemoveWorkloads specifies clean uninstall
	CDIUninstallStrategyRemoveWorkloads CDIUninstallStrategy = "RemoveWorkloads"

	// CDIUninstallStrategyBlockUninstallIfWorkloadsExist "leaves stuff around"
	CDIUninstallStrategyBlockUninstallIfWorkloadsExist CDIUninstallStrategy = "BlockUninstallIfWorkloadsExist"
)

// CDIPhase is the current phase of the CDI deployment
type CDIPhase string

// CDIStatus defines the status of the installation
type CDIStatus struct {
	sdkapi.Status `json:",inline"`
}

const (
	// CDIPhaseDeploying signals that the CDI resources are being deployed
	CDIPhaseDeploying CDIPhase = "Deploying"

	// CDIPhaseDeployed signals that the CDI resources are successflly deployed
	CDIPhaseDeployed CDIPhase = "Deployed"

	// CDIPhaseDeleting signals that the CDI resources are being removed
	CDIPhaseDeleting CDIPhase = "Deleting"

	// CDIPhaseDeleted signals that the CDI resources are deleted
	CDIPhaseDeleted CDIPhase = "Deleted"

	// CDIPhaseError signals that the CDI deployment is in an error state
	CDIPhaseError CDIPhase = "Error"

	// CDIPhaseUpgrading signals that the CDI resources are being deployed
	CDIPhaseUpgrading CDIPhase = "Upgrading"

	// CDIPhaseEmpty is an uninitialized phase
	CDIPhaseEmpty CDIPhase = ""
)

//CDIList provides the needed parameters to do request a list of CDIs from the system
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CDIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// Items provides a list of CDIs
	Items []CDI `json:"items"`
}

// this has to be here otherwise informer-gen doesn't recognize it
// see https://github.com/kubernetes/code-generator/issues/59
// +genclient:nonNamespaced

// CDIConfig provides a user configuration for CDI
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type CDIConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CDIConfigSpec   `json:"spec"`
	Status CDIConfigStatus `json:"status,omitempty"`
}

//Percent is a string that can only be a value between [0,1)
// (Note: we actually rely on reconcile to reject invalid values)
// +kubebuilder:validation:Pattern=`^(0(?:\.\d{1,3})?|1)$`
type Percent string

//FilesystemOverhead defines the reserved size for PVCs with VolumeMode: Filesystem
type FilesystemOverhead struct {
	// Global is how much space of a Filesystem volume should be reserved for overhead. This value is used unless overridden by a more specific value (per storageClass)
	Global Percent `json:"global,omitempty"`
	// StorageClass specifies how much space of a Filesystem volume should be reserved for safety. The keys are the storageClass and the values are the overhead. This value overrides the global value
	StorageClass map[string]Percent `json:"storageClass,omitempty"`
}

//CDIConfigSpec defines specification for user configuration
type CDIConfigSpec struct {
	// Override the URL used when uploading to a DataVolume
	UploadProxyURLOverride *string `json:"uploadProxyURLOverride,omitempty"`
	// ImportProxy contains importer pod proxy configuration.
	// +optional
	ImportProxy *ImportProxy `json:"importProxy,omitempty"`
	// Override the storage class to used for scratch space during transfer operations. The scratch space storage class is determined in the following order: 1. value of scratchSpaceStorageClass, if that doesn't exist, use the default storage class, if there is no default storage class, use the storage class of the DataVolume, if no storage class specified, use no storage class for scratch space
	ScratchSpaceStorageClass *string `json:"scratchSpaceStorageClass,omitempty"`
	// ResourceRequirements describes the compute resource requirements.
	PodResourceRequirements *corev1.ResourceRequirements `json:"podResourceRequirements,omitempty"`
	// FeatureGates are a list of specific enabled feature gates
	FeatureGates []string `json:"featureGates,omitempty"`
	// FilesystemOverhead describes the space reserved for overhead when using Filesystem volumes. A value is between 0 and 1, if not defined it is 0.055 (5.5% overhead)
	FilesystemOverhead *FilesystemOverhead `json:"filesystemOverhead,omitempty"`
	// Preallocation controls whether storage for DataVolumes should be allocated in advance.
	Preallocation *bool `json:"preallocation,omitempty"`
	// InsecureRegistries is a list of TLS disabled registries
	InsecureRegistries []string `json:"insecureRegistries,omitempty"`
}

//CDIConfigStatus provides the most recently observed status of the CDI Config resource
type CDIConfigStatus struct {
	// The calculated upload proxy URL
	UploadProxyURL *string `json:"uploadProxyURL,omitempty"`
	// ImportProxy contains importer pod proxy configuration.
	// +optional
	ImportProxy *ImportProxy `json:"importProxy,omitempty"`
	// The calculated storage class to be used for scratch space
	ScratchSpaceStorageClass string `json:"scratchSpaceStorageClass,omitempty"`
	// ResourceRequirements describes the compute resource requirements.
	DefaultPodResourceRequirements *corev1.ResourceRequirements `json:"defaultPodResourceRequirements,omitempty"`
	// FilesystemOverhead describes the space reserved for overhead when using Filesystem volumes. A percentage value is between 0 and 1
	FilesystemOverhead *FilesystemOverhead `json:"filesystemOverhead,omitempty"`
	// Preallocation controls whether storage for DataVolumes should be allocated in advance.
	Preallocation bool `json:"preallocation,omitempty"`
}

//CDIConfigList provides the needed parameters to do request a list of CDIConfigs from the system
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CDIConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// Items provides a list of CDIConfigs
	Items []CDIConfig `json:"items"`
}

//ImportProxy provides the information on how to configure the importer pod proxy.
type ImportProxy struct {
	// HTTPProxy is the URL http://<username>:<pswd>@<ip>:<port> of the import proxy for HTTP requests.  Empty means unset and will not result in the import pod env var.
	// +optional
	HTTPProxy *string `json:"HTTPProxy,omitempty"`
	// HTTPSProxy is the URL https://<username>:<pswd>@<ip>:<port> of the import proxy for HTTPS requests.  Empty means unset and will not result in the import pod env var.
	// +optional
	HTTPSProxy *string `json:"HTTPSProxy,omitempty"`
	// NoProxy is a comma-separated list of hostnames and/or CIDRs for which the proxy should not be used. Empty means unset and will not result in the import pod env var.
	// +optional
	NoProxy *string `json:"noProxy,omitempty"`
	// TrustedCAProxy is the name of a ConfigMap in the cdi namespace that contains a user-provided trusted certificate authority (CA) bundle.
	// The TrustedCAProxy field is consumed by the import controller that is resposible for coping it to a config map named trusted-ca-proxy-bundle-cm in the cdi namespace.
	// Here is an example of the ConfigMap (in yaml):
	//
	// apiVersion: v1
	// kind: ConfigMap
	// metadata:
	//   name: trusted-ca-proxy-bundle-cm
	//   namespace: cdi
	// data:
	//   ca.pem: |
	//     -----BEGIN CERTIFICATE-----
	// 	   ... <base64 encoded cert> ...
	// 	   -----END CERTIFICATE-----
	// +optional
	TrustedCAProxy *string `json:"trustedCAProxy,omitempty"`
}
