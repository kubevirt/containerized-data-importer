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
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/appscode/jsonpatch"
	restful "github.com/emicklei/go-restful"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	cdiuploadv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/upload/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/keys/keystest"
)

type testAuthorizer struct {
	allowed bool
	reason  string
	err     error
}

func (a *testAuthorizer) Authorize(req *restful.Request) (bool, string, error) {
	return a.allowed, a.reason, a.err
}

func signingKeySecretGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"cdi",
		apiSigningKeySecretName)
}

func signingKeySecretCreateAction(privateKey *rsa.PrivateKey) core.Action {
	secret, _ := keystest.NewPrivateKeySecret("cdi", apiSigningKeySecretName, privateKey)
	return core.NewCreateAction(
		schema.GroupVersionResource{
			Resource: "secrets",
			Version:  "v1",
		},
		"cdi",
		secret)
}

func cdiConfigGetAction() core.Action {
	return core.NewGetAction(
		schema.GroupVersionResource{
			Resource: "configmaps",
			Version:  "v1",
		},
		"cdi",
		"cdi-config")
}

func checkActions(expected []core.Action, actual []core.Action, t *testing.T) {
	for i, action := range actual {
		if len(expected) < i+1 {
			t.Errorf("%d unexpected actions: %+v", len(actual)-len(expected), actual[i:])
			break
		}

		expectedAction := expected[i]
		checkAction(expectedAction, action, t)
	}

	if len(expected) != len(actual) {
		t.Errorf("Expected %d actions, got %d", len(expected), len(actual))
	}
}

func printJSONDiff(objA, objB interface{}) string {
	aBytes, _ := json.Marshal(objA)
	bBytes, _ := json.Marshal(objB)
	patches, _ := jsonpatch.CreatePatch(aBytes, bBytes)
	pBytes, _ := json.Marshal(patches)
	return string(pBytes)
}

func checkAction(expected, actual core.Action, t *testing.T) {
	if !(expected.Matches(actual.GetVerb(), actual.GetResource().Resource) && actual.GetSubresource() == expected.GetSubresource()) {
		t.Errorf("Expected\n\t%#v\ngot\n\t%#v", expected, actual)
		return
	}

	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("Action has wrong type. Expected: %t. Got: %t", expected, actual)
		return
	}

	switch a := actual.(type) {
	case core.CreateAction:
		e, _ := expected.(core.CreateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, printJSONDiff(expObject, object))
		}
	case core.UpdateAction:
		e, _ := expected.(core.UpdateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		if !reflect.DeepEqual(expObject, object) {
			t.Errorf("Action %s %s has wrong object\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, printJSONDiff(expObject, object))
		}
	case core.PatchAction:
		e, _ := expected.(core.PatchAction)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		if !reflect.DeepEqual(expPatch, expPatch) {
			t.Errorf("Action %s %s has wrong patch\nDiff:\n %s",
				a.GetVerb(), a.GetResource().Resource, printJSONDiff(expPatch, patch))
		}
	}
}

func generateTestKey() (*rsa.PrivateKey, error) {
	apiKeyPair, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return apiKeyPair, nil
}

func TestGetSelfSignedCert(t *testing.T) {
	signingKey, err := generateTestKey()
	if err != nil {
		t.Errorf("error generating keys: %v", err)
	}

	signingKeySecret, err := keystest.NewPrivateKeySecret("cdi", apiSigningKeySecretName, signingKey)
	if err != nil {
		t.Errorf("error creating secret: %v", err)
	}

	kubeobjects := []runtime.Object{}
	kubeobjects = append(kubeobjects, signingKeySecret)

	actions := []core.Action{}
	actions = append(actions, signingKeySecretGetAction())

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	app := &cdiAPIApp{
		client: client,
	}

	err = app.getKeysAndCerts()
	if err != nil {
		t.Errorf("error creating upload api app: %v", err)
	}

	checkActions(actions, client.Actions(), t)
}

func TestShouldGenerateCertsAndKeyFirstRun(t *testing.T) {
	kubeobjects := []runtime.Object{}

	client := k8sfake.NewSimpleClientset(kubeobjects...)

	app := &cdiAPIApp{
		client: client,
	}

	err := app.getKeysAndCerts()
	if err != nil {
		t.Errorf("error creating upload api app: %v", err)
	}

	actions := []core.Action{}
	actions = append(actions, signingKeySecretGetAction())
	actions = append(actions, cdiConfigGetAction())
	actions = append(actions, signingKeySecretCreateAction(app.privateSigningKey))

	checkActions(actions, client.Actions(), t)
}

func TestGetAPIResouceList(t *testing.T) {
	rr := doGetRequest(t, "/apis/upload.cdi.kubevirt.io/v1alpha1")

	resourceList := metav1.APIResourceList{}
	err := json.Unmarshal(rr.Body.Bytes(), &resourceList)
	if err != nil {
		t.Errorf("Couldn't convert to object from JSON: %+v", err)
	}

	expectedResourceList := metav1.APIResourceList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "APIResourceList",
			APIVersion: "v1",
		},
		GroupVersion: "upload.cdi.kubevirt.io/v1alpha1",
		APIResources: []metav1.APIResource{
			{
				Name:         "uploadtokenrequests",
				SingularName: "uploadtokenrequest",
				Namespaced:   true,
				Group:        "upload.cdi.kubevirt.io",
				Version:      "v1alpha1",
				Kind:         "UploadTokenRequest",
				Verbs:        []string{"create"},
				ShortNames:   []string{"utr", "utrs"},
			},
		},
	}

	checkEqual(t, expectedResourceList, resourceList)
}

func TestGetAPIGroup(t *testing.T) {
	rr := doGetRequest(t, "/apis/upload.cdi.kubevirt.io")

	apiGroup := metav1.APIGroup{}
	err := json.Unmarshal(rr.Body.Bytes(), &apiGroup)
	if err != nil {
		t.Errorf("Couldn't convert to object from JSON: %+v", err)
	}

	expectedAPIGroup := getExpectedAPIGroup()

	checkEqual(t, expectedAPIGroup, apiGroup)
}

func TestGetRootPaths(t *testing.T) {
	rr := doGetRequest(t, "/")

	rootPaths := metav1.RootPaths{}
	err := json.Unmarshal(rr.Body.Bytes(), &rootPaths)
	if err != nil {
		t.Errorf("Couldn't convert to object from JSON: %+v", err)
	}

	expectedRootPaths := metav1.RootPaths{
		Paths: []string{
			"/apis",
			"/apis/",
			"/apis/upload.cdi.kubevirt.io",
			"/apis/upload.cdi.kubevirt.io/v1alpha1",
			"/healthz",
		},
	}

	checkEqual(t, expectedRootPaths, rootPaths)
}

func TestGetAPIGroupList(t *testing.T) {
	rr := doGetRequest(t, "/apis")

	apiGroupList := metav1.APIGroupList{}
	err := json.Unmarshal(rr.Body.Bytes(), &apiGroupList)
	if err != nil {
		t.Errorf("Couldn't convert to object from JSON: %+v", err)
	}

	expectedAPIGroupList := metav1.APIGroupList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "APIGroupList",
			APIVersion: "v1",
		},
		Groups: []metav1.APIGroup{
			getExpectedAPIGroup(),
		},
	}

	checkEqual(t, expectedAPIGroupList, apiGroupList)
}

func TestHealthz(t *testing.T) {
	rr := doGetRequest(t, "/healthz")

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("/healthz returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestGetToken(t *testing.T) {
	type args struct {
		authorizer     CdiAPIAuthorizer
		pvc            *v1.PersistentVolumeClaim
		uploadPossible uploadPossibleFunc
	}

	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	request := &cdiuploadv1alpha1.UploadTokenRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-token",
			Namespace: "default",
		},
		Spec: cdiuploadv1alpha1.UploadTokenRequestSpec{
			PvcName: "test-pvc",
		},
	}

	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "default",
		},
	}

	serializedRequest, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	authorizeSuccess := &testAuthorizer{allowed: true}

	tests := []struct {
		name           string
		args           args
		expectedStatus int
		checkToken     bool
	}{
		{
			"authoriser error",
			args{
				authorizer: &testAuthorizer{allowed: false, reason: "", err: fmt.Errorf("Error")},
			},
			http.StatusInternalServerError,
			false,
		},
		{
			"authoriser not allowed",
			args{
				authorizer: &testAuthorizer{allowed: false, reason: "bad person", err: nil},
			},
			http.StatusUnauthorized,
			false,
		},
		{
			"pvc does not exist",
			args{
				authorizer: authorizeSuccess,
			},
			http.StatusBadRequest,
			false,
		},
		{
			"upload not possible",
			args{
				authorizer:     authorizeSuccess,
				pvc:            pvc,
				uploadPossible: func(*v1.PersistentVolumeClaim) error { return fmt.Errorf("NOPE") },
			},
			http.StatusServiceUnavailable,
			false,
		},
		{
			"upload possible",
			args{
				authorizer:     authorizeSuccess,
				pvc:            pvc,
				uploadPossible: func(*v1.PersistentVolumeClaim) error { return nil },
			},
			http.StatusOK,
			true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			kubeobjects := []runtime.Object{}
			if test.args.pvc != nil {
				kubeobjects = append(kubeobjects, test.args.pvc)
			}
			client := k8sfake.NewSimpleClientset(kubeobjects...)

			app := &cdiAPIApp{client: client,
				privateSigningKey: signingKey,
				authorizer:        test.args.authorizer,
				uploadPossible:    test.args.uploadPossible,
				tokenGenerator:    newUploadTokenGenerator(signingKey)}
			app.composeUploadTokenAPI()

			req, _ := http.NewRequest("POST",
				"/apis/upload.cdi.kubevirt.io/v1alpha1/namespaces/default/uploadtokenrequests",
				bytes.NewReader(serializedRequest))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			app.container.ServeHTTP(rr, req)

			if rr.Code != test.expectedStatus {
				tt.Fatalf("Wrong status code, expected %d, got %d", test.expectedStatus, rr.Code)
			}

			if test.checkToken {
				uploadTokenRequest := &cdiuploadv1alpha1.UploadTokenRequest{}
				err := json.Unmarshal(rr.Body.Bytes(), &uploadTokenRequest)
				if err != nil {
					tt.Fatalf("Deserializing UploadTokenRequest failed: %+v", err)
				}

				if uploadTokenRequest.Status.Token == "" {
					tt.Fatal("UploadTokenRequest response does not contain a token")
				}
			}
		})
	}
}

func doGetRequest(t *testing.T, url string) *httptest.ResponseRecorder {
	app := &cdiAPIApp{}
	app.composeUploadTokenAPI()

	req, _ := http.NewRequest("GET", url, nil)
	rr := httptest.NewRecorder()

	app.container.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Wrong status code, expected %d, got %d", http.StatusOK, rr.Code)
	}

	return rr
}

func getExpectedAPIGroup() metav1.APIGroup {
	return metav1.APIGroup{
		Name: "upload.cdi.kubevirt.io",
		TypeMeta: metav1.TypeMeta{
			Kind:       "APIGroup",
			APIVersion: "v1",
		},
		PreferredVersion: metav1.GroupVersionForDiscovery{
			GroupVersion: "upload.cdi.kubevirt.io/v1alpha1",
			Version:      "v1alpha1",
		},
		Versions: []metav1.GroupVersionForDiscovery{
			{
				GroupVersion: "upload.cdi.kubevirt.io/v1alpha1",
				Version:      "v1alpha1",
			},
		},
		ServerAddressByClientCIDRs: []metav1.ServerAddressByClientCIDR{
			{
				ClientCIDR:    "0.0.0.0/0",
				ServerAddress: "",
			},
		},
	}
}

func checkEqual(t *testing.T, expected, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Objects are not equal\nDiff:\n %s", diff.ObjectGoPrintDiff(expected, actual))
	}
}
