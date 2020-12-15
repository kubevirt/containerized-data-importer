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
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/certrotation"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/kubernetes"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	cdicerts "kubevirt.io/containerized-data-importer/pkg/operator/resources/cert"
)

const (
	annCertConfig = "operator.cdi.kubevirt.io/certConfig"
)

// CertManager is the client interface to the certificate manager/refresher
type CertManager interface {
	Sync(certs []cdicerts.CertificateDefinition) error
}

type certListers struct {
	secretLister    listerscorev1.SecretLister
	configMapLister listerscorev1.ConfigMapLister
}

type certManager struct {
	namespaces []string
	listerMap  map[string]*certListers

	k8sClient     kubernetes.Interface
	informers     v1helpers.KubeInformersForNamespaces
	eventRecorder events.Recorder
}

type serializedCertConfig struct {
	Lifetime string `json:"lifetime,omitempty"`
	Refresh  string `json:"refresh,omitempty"`
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

		if cm.listerMap == nil {
			cm.listerMap = make(map[string]*certListers)
		}

		cm.listerMap[ns] = &certListers{
			secretLister:    cm.informers.InformersFor(ns).Core().V1().Secrets().Lister(),
			configMapLister: cm.informers.InformersFor(ns).Core().V1().ConfigMaps().Lister(),
		}
	}

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

func (cm *certManager) ensureCertConfig(secret *corev1.Secret, certConfig cdicerts.CertificateConfig) (*corev1.Secret, error) {
	scc := &serializedCertConfig{
		Lifetime: certConfig.Lifetime.String(),
		Refresh:  certConfig.Refresh.String(),
	}

	configBytes, err := json.Marshal(scc)
	if err != nil {
		return nil, err
	}

	configString := string(configBytes)
	currentConfig := secret.Annotations[annCertConfig]
	if currentConfig == configString {
		return secret, nil
	}

	secretCpy := secret.DeepCopy()

	if secretCpy.Annotations == nil {
		secretCpy.Annotations = make(map[string]string)
	}

	// force refresh
	if _, ok := secretCpy.Annotations[certrotation.CertificateNotAfterAnnotation]; ok {
		secretCpy.Annotations[certrotation.CertificateNotAfterAnnotation] = time.Now().Format(time.RFC3339)
	}
	secretCpy.Annotations[annCertConfig] = configString

	if secret, err = cm.k8sClient.CoreV1().Secrets(secretCpy.Namespace).Update(context.TODO(), secretCpy, metav1.UpdateOptions{}); err != nil {
		return nil, err
	}

	return secret, nil
}

func (cm *certManager) createSecret(namespace, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	return cm.k8sClient.CoreV1().Secrets(namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
}

func (cm *certManager) ensureSigner(cd cdicerts.CertificateDefinition) (*crypto.CA, error) {
	listers, ok := cm.listerMap[cd.SignerSecret.Namespace]
	if !ok {
		return nil, fmt.Errorf("no lister for namespace %s", cd.SignerSecret.Namespace)
	}
	lister := listers.secretLister
	secret, err := lister.Secrets(cd.SignerSecret.Namespace).Get(cd.SignerSecret.Name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}

		secret, err = cm.createSecret(cd.SignerSecret.Namespace, cd.SignerSecret.Name)
		if err != nil {
			return nil, err
		}
	}

	if secret, err = cm.ensureCertConfig(secret, cd.SignerConfig); err != nil {
		return nil, err
	}

	sr := certrotation.SigningRotation{
		Name:          secret.Name,
		Namespace:     secret.Namespace,
		Validity:      cd.SignerConfig.Lifetime,
		Refresh:       cd.SignerConfig.Refresh,
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
	listers, ok := cm.listerMap[configMap.Namespace]
	if !ok {
		return nil, fmt.Errorf("no lister for namespace %s", configMap.Namespace)
	}
	lister := listers.configMapLister
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
	listers, ok := cm.listerMap[cd.SignerSecret.Namespace]
	if !ok {
		return fmt.Errorf("no lister for namespace %s", cd.SignerSecret.Namespace)
	}
	lister := listers.secretLister
	secret, err := lister.Secrets(cd.TargetSecret.Namespace).Get(cd.TargetSecret.Name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		secret, err = cm.createSecret(cd.TargetSecret.Namespace, cd.TargetSecret.Name)
		if err != nil {
			return err
		}
	}

	if secret, err = cm.ensureCertConfig(secret, cd.TargetConfig); err != nil {
		return err
	}

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

	tr := certrotation.TargetRotation{
		Name:          secret.Name,
		Namespace:     secret.Namespace,
		Validity:      cd.TargetConfig.Lifetime,
		Refresh:       cd.TargetConfig.Refresh,
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
