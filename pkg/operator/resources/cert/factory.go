/*
Copyright 2020 The CDI Authors.

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

package cert

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

const day = 24 * time.Hour

// FactoryArgs contains the required parameters to generate certs
type FactoryArgs struct {
	Namespace string
}

// CertificateDefinition contains the data required to create/manage certtificate chains
type CertificateDefinition struct {
	// current CA key/cert
	SignerSecret   *corev1.Secret
	SignerValidity time.Duration
	SignerRefresh  time.Duration

	// all valid CA certs
	CertBundleConfigmap *corev1.ConfigMap

	// current key/cert for target
	TargetSecret   *corev1.Secret
	TargetValidity time.Duration
	TargetRefresh  time.Duration

	// only one of the following should be set
	// contains target key/cert for server
	TargetService *string
	// contains target user name
	TargetUser *string
}

// CreateCertificateDefinitions creates certificate definitions
func CreateCertificateDefinitions(args *FactoryArgs) []CertificateDefinition {
	defs := createCertificateDefinitions()
	for _, def := range defs {
		if def.SignerSecret != nil {
			addNamespace(args.Namespace, def.SignerSecret)
		}

		if def.CertBundleConfigmap != nil {
			addNamespace(args.Namespace, def.CertBundleConfigmap)
		}

		if def.TargetSecret != nil {
			addNamespace(args.Namespace, def.TargetSecret)
		}
	}

	return defs
}

func addNamespace(namespace string, obj metav1.Object) {
	if obj.GetNamespace() == "" {
		obj.SetNamespace(namespace)
	}
}

func createCertificateDefinitions() []CertificateDefinition {
	return []CertificateDefinition{
		{
			SignerSecret:        createSecret("cdi-apiserver-signer"),
			SignerValidity:      30 * day,
			SignerRefresh:       15 * day,
			CertBundleConfigmap: createConfigMap("cdi-apiserver-signer-bundle"),
			TargetSecret:        createSecret("cdi-apiserver-server-cert"),
			TargetValidity:      48 * time.Hour,
			TargetRefresh:       24 * time.Hour,
			TargetService:       &[]string{"cdi-api"}[0],
		},
		{
			SignerSecret:        createSecret("cdi-uploadproxy-signer"),
			SignerValidity:      30 * day,
			SignerRefresh:       15 * day,
			CertBundleConfigmap: createConfigMap("cdi-uploadproxy-signer-bundle"),
			TargetSecret:        createSecret("cdi-uploadproxy-server-cert"),
			TargetValidity:      48 * time.Hour,
			TargetRefresh:       24 * time.Hour,
			TargetService:       &[]string{"cdi-uploadproxy"}[0],
		},
		{
			SignerSecret:        createSecret("cdi-uploadserver-signer"),
			SignerValidity:      10 * 365 * day,
			SignerRefresh:       8 * 365 * day,
			CertBundleConfigmap: createConfigMap("cdi-uploadserver-signer-bundle"),
		},
		{
			SignerSecret:        createSecret("cdi-uploadserver-client-signer"),
			SignerValidity:      10 * 365 * day,
			SignerRefresh:       8 * 365 * day,
			CertBundleConfigmap: createConfigMap("cdi-uploadserver-client-signer-bundle"),
			TargetSecret:        createSecret("cdi-uploadserver-client-cert"),
			TargetValidity:      48 * time.Hour,
			TargetRefresh:       24 * time.Hour,
			TargetUser:          &[]string{"client.upload-server.cdi.kubevirt.io"}[0],
		},
	}
}

func createSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: utils.ResourcesBuiler.WithCommonLabels(nil),
		},
	}
}

func createConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: utils.ResourcesBuiler.WithCommonLabels(nil),
		},
	}
}
