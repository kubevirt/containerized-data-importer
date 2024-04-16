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
 * Copyright 2022 Red Hat, Inc.
 *
 */

package tlscryptowatch

import (
	"context"
	"crypto/tls"
	"sync"

	ocpcrypto "github.com/openshift/library-go/pkg/crypto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	informers "kubevirt.io/containerized-data-importer/pkg/client/informers/externalversions"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

// CryptoConfig contains TLS crypto configurables
type CryptoConfig struct {
	CipherSuites []uint16
	MinVersion   uint16
}

// CdiConfigTLSWatcher is the interface of cdiConfigTLSWatcher
type CdiConfigTLSWatcher interface {
	GetCdiTLSConfig() *CryptoConfig
	GetInformer() cache.SharedIndexInformer
}

type cdiConfigTLSWatcher struct {
	// keep this around for tests
	informer cache.SharedIndexInformer

	config *CryptoConfig
	mutex  sync.RWMutex
}

// NewCdiConfigTLSWatcher crates a new cdiConfigTLSWatcher
func NewCdiConfigTLSWatcher(ctx context.Context, cdiClient cdiclient.Interface) (CdiConfigTLSWatcher, error) {
	cdiInformerFactory := informers.NewFilteredSharedInformerFactory(cdiClient,
		common.DefaultResyncPeriod,
		metav1.NamespaceAll,
		func(options *metav1.ListOptions) {
			options.FieldSelector = "metadata.name=" + common.ConfigName
		},
	)

	cdiConfigInformer := cdiInformerFactory.Cdi().V1beta1().CDIConfigs().Informer()

	ctw := &cdiConfigTLSWatcher{
		informer: cdiConfigInformer,
		config:   DefaultCryptoConfig(),
	}

	_, err := cdiConfigInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.V(3).Infof("cdiConfigInformer add callback: %+v", obj)
			ctw.updateConfig(obj.(*cdiv1.CDIConfig))
		},
		UpdateFunc: func(_, obj interface{}) {
			klog.V(3).Infof("cdiConfigInformer update callback: %+v", obj)
			ctw.updateConfig(obj.(*cdiv1.CDIConfig))
		},
		DeleteFunc: func(obj interface{}) {
			config := obj.(*cdiv1.CDIConfig)
			klog.Errorf("CDIConfig %s deleted", config.Name)
		},
	})

	if err != nil {
		return nil, err
	}

	go cdiInformerFactory.Start(ctx.Done())

	klog.V(3).Infoln("Waiting for cache sync")
	cache.WaitForCacheSync(ctx.Done(), cdiConfigInformer.HasSynced)
	klog.V(3).Infoln("Cache sync complete")

	return ctw, nil
}

func (ctw *cdiConfigTLSWatcher) GetCdiTLSConfig() *CryptoConfig {
	ctw.mutex.RLock()
	defer ctw.mutex.RUnlock()
	return ctw.config
}

func (ctw *cdiConfigTLSWatcher) GetInformer() cache.SharedIndexInformer {
	return ctw.informer
}

func (ctw *cdiConfigTLSWatcher) updateConfig(config *cdiv1.CDIConfig) {
	newConfig := &CryptoConfig{}

	cipherNames, minTypedTLSVersion := SelectCipherSuitesAndMinTLSVersion(config.Spec.TLSSecurityProfile)
	minTLSVersion, _ := ocpcrypto.TLSVersion(string(minTypedTLSVersion))
	ciphers := CipherSuitesIDs(cipherNames)
	newConfig.CipherSuites = ciphers
	newConfig.MinVersion = minTLSVersion

	ctw.mutex.Lock()
	defer ctw.mutex.Unlock()
	ctw.config = newConfig
}

// SelectCipherSuitesAndMinTLSVersion returns cipher names and minimal TLS version according to the input profile
func SelectCipherSuitesAndMinTLSVersion(profile *cdiv1.TLSSecurityProfile) ([]string, cdiv1.TLSProtocolVersion) {
	if profile == nil {
		profile = &cdiv1.TLSSecurityProfile{
			Type:         cdiv1.TLSProfileIntermediateType,
			Intermediate: &cdiv1.IntermediateTLSProfile{},
		}
	}

	if profile.Custom != nil {
		return profile.Custom.TLSProfileSpec.Ciphers, profile.Custom.TLSProfileSpec.MinTLSVersion
	}

	return cdiv1.TLSProfiles[profile.Type].Ciphers, cdiv1.TLSProfiles[profile.Type].MinTLSVersion
}

// DefaultCryptoConfig returns a crypto config with legitimate defaults to start with
func DefaultCryptoConfig() *CryptoConfig {
	defaultType := cdiv1.TLSProfileIntermediateType
	minTLSVersion, _ := ocpcrypto.TLSVersion(string(cdiv1.TLSProfiles[defaultType].MinTLSVersion))
	ciphers := CipherSuitesIDs(cdiv1.TLSProfiles[defaultType].Ciphers)

	return &CryptoConfig{
		CipherSuites: ciphers,
		MinVersion:   minTLSVersion,
	}
}

// CipherSuitesIDs translates cipher names to IDs which can be straight to the tls.Config
func CipherSuitesIDs(names []string) []uint16 {
	// ref: https://www.iana.org/assignments/tls-parameters/tls-parameters.xml
	var idByName = map[string]uint16{
		// TLS 1.2
		"ECDHE-ECDSA-AES128-GCM-SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"ECDHE-RSA-AES128-GCM-SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"ECDHE-ECDSA-AES256-GCM-SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		"ECDHE-RSA-AES256-GCM-SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		"ECDHE-ECDSA-CHACHA20-POLY1305": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		"ECDHE-RSA-CHACHA20-POLY1305":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		"ECDHE-ECDSA-AES128-SHA256":     tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		"ECDHE-RSA-AES128-SHA256":       tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		"AES128-GCM-SHA256":             tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		"AES256-GCM-SHA384":             tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		"AES128-SHA256":                 tls.TLS_RSA_WITH_AES_128_CBC_SHA256,

		// TLS 1
		"ECDHE-ECDSA-AES128-SHA": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		"ECDHE-RSA-AES128-SHA":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		"ECDHE-ECDSA-AES256-SHA": tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		"ECDHE-RSA-AES256-SHA":   tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,

		// SSL 3
		"AES128-SHA":   tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		"AES256-SHA":   tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		"DES-CBC3-SHA": tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	}
	for _, cipherSuite := range tls.CipherSuites() {
		idByName[cipherSuite.Name] = cipherSuite.ID
	}

	ids := []uint16{}
	for _, name := range names {
		if id, ok := idByName[name]; ok {
			ids = append(ids, id)
		}
	}

	return ids
}
