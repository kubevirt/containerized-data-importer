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

package controller

import (
	"crypto/x509"
	"fmt"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/certrotation"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/kubernetes"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	cdicerts "kubevirt.io/containerized-data-importer/pkg/operator/resources/cert"
)

// CertManager is the client interface to the certificate manager/refresher
type CertManager interface {
	Sync(certs []cdicerts.CertificateDefinition) error
}

type certManager struct {
	namespaces []string

	k8sClient     kubernetes.Interface
	informers     v1helpers.KubeInformersForNamespaces
	eventRecorder events.Recorder
}

// NewCertManager creates a new certificate manager/refresher
func NewCertManager(mgr manager.Manager, installNamespace string, additionalNamespaces ...string) (CertManager, error) {
	k8sClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, err
	}

	cm := newCertManager(k8sClient, installNamespace, additionalNamespaces...)

	// so we can start caches
	if err = mgr.Add(cm); err != nil {
		return nil, err
	}

	return cm, nil
}

func newCertManager(client kubernetes.Interface, installNamespace string, additionalNamespaces ...string) *certManager {
	namespaces := append(additionalNamespaces, installNamespace)
	informers := v1helpers.NewKubeInformersForNamespaces(client, namespaces...)

	controllerRef, err := events.GetControllerReferenceForCurrentPod(client, installNamespace, nil)
	if err != nil {
		log.Info("Unable to get controller reference, using namespace")
	}

	eventRecorder := events.NewRecorder(client.CoreV1().Events(installNamespace), installNamespace, controllerRef)

	return &certManager{
		namespaces:    namespaces,
		k8sClient:     client,
		informers:     informers,
		eventRecorder: eventRecorder,
	}
}

func (cm *certManager) Start(stopCh <-chan struct{}) error {
	cm.informers.Start(stopCh)

	for _, ns := range cm.namespaces {
		secretInformer := cm.informers.InformersFor(ns).Core().V1().Secrets().Informer()
		go secretInformer.Run(stopCh)

		configMapInformer := cm.informers.InformersFor(ns).Core().V1().ConfigMaps().Informer()
		go configMapInformer.Run(stopCh)

		if !toolscache.WaitForCacheSync(stopCh, secretInformer.HasSynced, configMapInformer.HasSynced) {
			return fmt.Errorf("could not sync informer cache")
		}
	}

	<-stopCh

	return nil
}

func (cm *certManager) Sync(certs []cdicerts.CertificateDefinition) error {
	for _, cd := range certs {
		ca, err := cm.ensureSigner(cd)
		if err != nil {
			return err
		}

		if cd.CertBundleConfigmap == nil {
			continue
		}

		bundle, err := cm.ensureCertBundle(cd, ca)
		if err != nil {
			return err
		}

		if cd.TargetSecret == nil {
			continue
		}

		if err := cm.ensureTarget(cd, ca, bundle); err != nil {
			return err
		}
	}

	return nil
}

func (cm *certManager) ensureSigner(cd cdicerts.CertificateDefinition) (*crypto.CA, error) {
	secret := cd.SignerSecret
	lister := cm.informers.InformersFor(secret.Namespace).Core().V1().Secrets().Lister()
	sr := certrotation.SigningRotation{
		Name:          secret.Name,
		Namespace:     secret.Namespace,
		Validity:      cd.SignerValidity,
		Refresh:       cd.SignerRefresh,
		Lister:        lister,
		Client:        cm.k8sClient.CoreV1(),
		EventRecorder: cm.eventRecorder,
	}

	ca, err := sr.EnsureSigningCertKeyPair()
	if err != nil {
		return nil, err
	}

	return ca, nil
}

func (cm *certManager) ensureCertBundle(cd cdicerts.CertificateDefinition, ca *crypto.CA) ([]*x509.Certificate, error) {
	configMap := cd.CertBundleConfigmap
	lister := cm.informers.InformersFor(configMap.Namespace).Core().V1().ConfigMaps().Lister()
	br := certrotation.CABundleRotation{
		Name:          configMap.Name,
		Namespace:     configMap.Namespace,
		Lister:        lister,
		Client:        cm.k8sClient.CoreV1(),
		EventRecorder: cm.eventRecorder,
	}

	certs, err := br.EnsureConfigMapCABundle(ca)
	if err != nil {
		return nil, err
	}

	return certs, nil
}

func (cm *certManager) ensureTarget(cd cdicerts.CertificateDefinition, ca *crypto.CA, bundle []*x509.Certificate) error {
	secret := cd.TargetSecret
	var targetCreator certrotation.TargetCertCreator
	if cd.TargetService != nil {
		targetCreator = &certrotation.ServingRotation{
			Hostnames: func() []string {
				return []string{
					*cd.TargetService,
					fmt.Sprintf("%s.%s", *cd.TargetService, secret.Namespace),
					fmt.Sprintf("%s.%s.svc", *cd.TargetService, secret.Namespace),
				}
			},
		}
	} else {
		targetCreator = &certrotation.ClientRotation{
			UserInfo: &user.DefaultInfo{Name: *cd.TargetUser},
		}
	}

	lister := cm.informers.InformersFor(secret.Namespace).Core().V1().Secrets().Lister()
	tr := certrotation.TargetRotation{
		Name:          secret.Name,
		Namespace:     secret.Namespace,
		Validity:      cd.TargetValidity,
		Refresh:       cd.TargetRefresh,
		CertCreator:   targetCreator,
		Lister:        lister,
		Client:        cm.k8sClient.CoreV1(),
		EventRecorder: cm.eventRecorder,
	}

	if err := tr.EnsureTargetCertKeyPair(ca, bundle); err != nil {
		return err
	}

	return nil
}
