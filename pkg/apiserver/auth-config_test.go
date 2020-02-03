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

package apiserver

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	k8scert "k8s.io/client-go/util/cert"
	aggregatorapifake "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/fake"

	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/triple"
)

func generateCACert(t *testing.T) string {
	keyPair, err := triple.NewCA(util.RandAlphaNum(10))
	if err != nil {
		t.Errorf("Error creating CA cert")
	}
	return string(cert.EncodeCertPEM(keyPair.Cert))
}

func getAPIServerConfigMap(t *testing.T) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "extension-apiserver-authentication",
			Namespace: "kube-system",
		},
		Data: map[string]string{
			"client-ca-file":                     generateCACert(t),
			"requestheader-allowed-names":        "[\"front-proxy-client\"]",
			"requestheader-client-ca-file":       generateCACert(t),
			"requestheader-extra-headers-prefix": "[\"X-Remote-Extra-\"]",
			"requestheader-group-headers":        "[\"X-Remote-Group\"]",
			"requestheader-username-headers":     "[\"X-Remote-User\"]",
		},
	}
}

func verifyAuthConfig(t *testing.T, cm *corev1.ConfigMap, authConfig *AuthConfig) {
	if !reflect.DeepEqual([]byte(cm.Data["client-ca-file"]), authConfig.ClientCABytes) {
		t.Errorf("client-ca-file not stored correctly")
	}

	if !reflect.DeepEqual([]byte(cm.Data["requestheader-client-ca-file"]), authConfig.RequestheaderClientCABytes) {
		t.Errorf("client-ca-file not stored correctly")
	}

	if !reflect.DeepEqual(deserializeStringSlice(cm.Data["requestheader-username-headers"]), authConfig.UserHeaders) {
		t.Errorf("requestheader-username-headers not stored correctly")
	}

	if !reflect.DeepEqual(deserializeStringSlice(cm.Data["requestheader-group-headers"]), authConfig.GroupHeaders) {
		t.Errorf("requestheader-group-headers not stored correctly")
	}

	if !reflect.DeepEqual(deserializeStringSlice(cm.Data["requestheader-extra-headers-prefix"]), authConfig.ExtraPrefixHeaders) {
		t.Errorf("requestheader-extra-headers-prefix not stored correctly")
	}
}

func TestNewCdiAPIServer(t *testing.T) {
	ch := make(chan struct{})
	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, getAPIServerConfigMap(t))

	client := k8sfake.NewSimpleClientset(kubeobjects...)
	aggregatorClient := aggregatorapifake.NewSimpleClientset()
	authorizer := &testAuthorizer{}
	authConfigWatcher := NewAuthConfigWatcher(client, ch)
	caBundle := []byte("data")

	server, err := NewCdiAPIServer("0.0.0.0", 0, client, aggregatorClient, authorizer, authConfigWatcher, caBundle, nil)
	if err != nil {
		t.Errorf("Upload api server creation failed: %+v", err)
	}

	app := server.(*cdiAPIApp)

	req, _ := http.NewRequest("GET", "/apis", nil)
	rr := httptest.NewRecorder()

	app.container.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Unexpected status code %d", rr.Code)
	}
}

func TestAuthConfigUpdate(t *testing.T) {
	ch := make(chan struct{})
	cm := getAPIServerConfigMap(t)
	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, cm)

	client := k8sfake.NewSimpleClientset(kubeobjects...)
	aggregatorClient := aggregatorapifake.NewSimpleClientset()
	authorizer := &testAuthorizer{}
	acw := NewAuthConfigWatcher(client, ch).(*authConfigWatcher)
	caBundle := []byte("data")

	server, err := NewCdiAPIServer("0.0.0.0", 0, client, aggregatorClient, authorizer, acw, caBundle, nil)
	if err != nil {
		t.Errorf("Upload api server creation failed: %+v", err)
	}

	app := server.(*cdiAPIApp)

	verifyAuthConfig(t, cm, app.authConfigWatcher.GetAuthConfig())

	cm.Data["requestheader-client-ca-file"] = generateCACert(t)

	cm, err = client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Update(cm)
	if err != nil {
		t.Errorf("Updating configmap failed: %+v", err)
	}

	// behavior of this changed in 16.4 used to wait then check so now explicitly waiting
	time.Sleep(100 * time.Millisecond)
	cache.WaitForCacheSync(ch, acw.informer.HasSynced)

	verifyAuthConfig(t, cm, app.authConfigWatcher.GetAuthConfig())
}

type fakeCertWatcher struct {
	cert *tls.Certificate
}

func (fcw *fakeCertWatcher) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return fcw.cert, nil
}

func NewFakeCertWatcher() CertWatcher {
	certBytes, keyBytes, _ := k8scert.GenerateSelfSignedCertKey("foobar", nil, nil)
	c, _ := tls.X509KeyPair(certBytes, keyBytes)
	return &fakeCertWatcher{cert: &c}
}

func TestGetTLSConfig(t *testing.T) {
	ch := make(chan struct{})
	cm := getAPIServerConfigMap(t)
	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, cm)

	client := k8sfake.NewSimpleClientset(kubeobjects...)
	aggregatorClient := aggregatorapifake.NewSimpleClientset()
	authorizer := &testAuthorizer{}
	acw := NewAuthConfigWatcher(client, ch).(*authConfigWatcher)
	caBundle := []byte("data")
	certWatcher := NewFakeCertWatcher()

	server, err := NewCdiAPIServer("0.0.0.0", 0, client, aggregatorClient, authorizer, acw, caBundle, certWatcher)
	if err != nil {
		t.Errorf("Upload api server creation failed: %+v", err)
	}

	app := server.(*cdiAPIApp)

	tlsConfig, err := app.getTLSConfig()
	if err != nil {
		t.Errorf("Error getting tls conig")
	}

	if !reflect.DeepEqual(tlsConfig.ClientCAs, acw.GetAuthConfig().CertPool) {
		t.Errorf("Client cert pools do not match")
	}
}
