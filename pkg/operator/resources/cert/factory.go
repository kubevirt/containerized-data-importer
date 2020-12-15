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

	SignerValidity *time.Duration
	SignerRefresh  *time.Duration

	TargetValidity *time.Duration
	TargetRefresh  *time.Duration
}

// CertificateConfig contains cert configuration data
type CertificateConfig struct {
	Lifetime time.Duration
	Refresh  time.Duration
}

// CertificateDefinition contains the data required to create/manage certtificate chains
type CertificateDefinition struct {
	// configurable by user
	Configurable bool

	// current CA key/cert
	SignerSecret *corev1.Secret
	SignerConfig CertificateConfig

	// all valid CA certs
	CertBundleConfigmap *corev1.ConfigMap

	// current key/cert for target
	TargetSecret *corev1.Secret
	TargetConfig CertificateConfig

	// only one of the following should be set
	// contains target key/cert for server
	TargetService *string
	// contains target user name
	TargetUser *string
}

// CreateCertificateDefinitions creates certificate definitions
func CreateCertificateDefinitions(args *FactoryArgs) []CertificateDefinition {
	defs := createCertificateDefinitions()
	for i := range defs {
		def := &defs[i]

		if def.SignerSecret != nil {
			addNamespace(args.Namespace, def.SignerSecret)
		}

		if def.CertBundleConfigmap != nil {
			addNamespace(args.Namespace, def.CertBundleConfigmap)
		}

		if def.TargetSecret != nil {
			addNamespace(args.Namespace, def.TargetSecret)
		}

		if def.Configurable {
			if args.SignerValidity != nil {
				def.SignerConfig.Lifetime = *args.SignerValidity
			}

			if args.SignerRefresh != nil {
				def.SignerConfig.Refresh = *args.SignerRefresh
			}

			if args.TargetValidity != nil {
				def.TargetConfig.Lifetime = *args.TargetValidity
			}

			if args.TargetRefresh != nil {
				def.TargetConfig.Refresh = *args.TargetRefresh
			}
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
			Configurable: true,
			SignerSecret: createSecret("cdi-apiserver-signer"),
			SignerConfig: CertificateConfig{
				Lifetime: 48 * time.Hour,
				Refresh:  24 * time.Hour,
			},
			CertBundleConfigmap: createConfigMap("cdi-apiserver-signer-bundle"),
			TargetSecret:        createSecret("cdi-apiserver-server-cert"),
			TargetConfig: CertificateConfig{
				Lifetime: 24 * time.Hour,
				Refresh:  12 * time.Hour,
			},
			TargetService: &[]string{"cdi-api"}[0],
		},
		{
			Configurable: true,
			SignerSecret: createSecret("cdi-uploadproxy-signer"),
			SignerConfig: CertificateConfig{
				Lifetime: 48 * day,
				Refresh:  24 * day,
			},
			CertBundleConfigmap: createConfigMap("cdi-uploadproxy-signer-bundle"),
			TargetSecret:        createSecret("cdi-uploadproxy-server-cert"),
			TargetConfig: CertificateConfig{
				Lifetime: 24 * time.Hour,
				Refresh:  12 * time.Hour,
			},
			TargetService: &[]string{"cdi-uploadproxy"}[0],
		},
		{
			SignerSecret: createSecret("cdi-uploadserver-signer"),
			SignerConfig: CertificateConfig{
				Lifetime: 10 * 365 * day,
				Refresh:  8 * 365 * day,
			},
			CertBundleConfigmap: createConfigMap("cdi-uploadserver-signer-bundle"),
		},
		{
			SignerSecret: createSecret("cdi-uploadserver-client-signer"),
			SignerConfig: CertificateConfig{
				Lifetime: 10 * 365 * day,
				Refresh:  8 * 365 * day,
			},
			CertBundleConfigmap: createConfigMap("cdi-uploadserver-client-signer-bundle"),
			TargetSecret:        createSecret("cdi-uploadserver-client-cert"),
			TargetConfig: CertificateConfig{
				Lifetime: 24 * time.Hour,
				Refresh:  12 * time.Hour,
			},
			TargetUser: &[]string{"client.upload-server.cdi.kubevirt.io"}[0],
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
