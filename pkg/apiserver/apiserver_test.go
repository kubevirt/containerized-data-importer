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

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/appscode/jsonpatch"
	restful "github.com/emicklei/go-restful"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"

	cdiuploadv1 "kubevirt.io/containerized-data-importer/pkg/apis/upload/v1beta1"
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

func checkActions(expected []core.Action, actual []core.Action) {
	for i, action := range actual {
		Expect(len(expected) >= i+1).To(BeTrue())

		expectedAction := expected[i]
		checkAction(expectedAction, action)
	}

	Expect(len(expected)).To(Equal(len(actual)))
}

func printJSONDiff(objA, objB interface{}) string {
	aBytes, _ := json.Marshal(objA)
	bBytes, _ := json.Marshal(objB)
	patches, _ := jsonpatch.CreatePatch(aBytes, bBytes)
	pBytes, _ := json.Marshal(patches)
	return string(pBytes)
}

func checkAction(expected, actual core.Action) {
	Expect(expected.Matches(actual.GetVerb(), actual.GetResource().Resource)).To(BeTrue())
	Expect(actual.GetSubresource()).To(Equal(expected.GetSubresource()))
	Expect(reflect.TypeOf(actual)).To(Equal(reflect.TypeOf(expected)))

	switch a := actual.(type) {
	case core.CreateAction:
		e, _ := expected.(core.CreateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		Expect(reflect.DeepEqual(expObject, object)).To(BeTrue())
	case core.UpdateAction:
		e, _ := expected.(core.UpdateAction)
		expObject := e.GetObject()
		object := a.GetObject()

		Expect(reflect.DeepEqual(expObject, object)).To(BeTrue())
	case core.PatchAction:
		e, _ := expected.(core.PatchAction)
		expPatch := e.GetPatch()
		patch := a.GetPatch()

		Expect(reflect.DeepEqual(expPatch, patch)).To(BeTrue())
	}
}

func generateTestKey() (*rsa.PrivateKey, error) {
	apiKeyPair, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return apiKeyPair, nil
}

func doGetRequest(url string) *httptest.ResponseRecorder {
	app := &cdiAPIApp{}
	app.composeUploadTokenAPI()

	req, err := http.NewRequest("GET", url, nil)
	Expect(err).ToNot(HaveOccurred())
	rr := httptest.NewRecorder()

	app.container.ServeHTTP(rr, req)

	status := rr.Code
	Expect(status).To(Equal(http.StatusOK))

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
			GroupVersion: "upload.cdi.kubevirt.io/v1beta1",
			Version:      "v1beta1",
		},
		Versions: []metav1.GroupVersionForDiscovery{
			{
				GroupVersion: "upload.cdi.kubevirt.io/v1beta1",
				Version:      "v1beta1",
			},
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

var _ = Describe("API server tests", func() {
	It("Get self-signed cert", func() {
		signingKey, err := generateTestKey()
		Expect(err).ToNot(HaveOccurred())

		signingKeySecret, err := keystest.NewPrivateKeySecret("cdi", apiSigningKeySecretName, signingKey)
		Expect(err).ToNot(HaveOccurred())

		kubeobjects := []runtime.Object{}
		kubeobjects = append(kubeobjects, signingKeySecret)

		actions := []core.Action{}
		actions = append(actions, signingKeySecretGetAction())

		client := k8sfake.NewSimpleClientset(kubeobjects...)

		app := &cdiAPIApp{
			client: client,
		}

		err = app.getKeysAndCerts()
		Expect(err).ToNot(HaveOccurred())

		checkActions(actions, client.Actions())
	})

	It("Should generate certs and key on first run", func() {
		client := k8sfake.NewSimpleClientset()

		app := &cdiAPIApp{
			client: client,
		}

		err := app.getKeysAndCerts()
		Expect(err).ToNot(HaveOccurred())

		actions := []core.Action{}
		actions = append(actions, signingKeySecretGetAction())
		actions = append(actions, cdiConfigGetAction())
		actions = append(actions, signingKeySecretCreateAction(app.privateSigningKey))

		checkActions(actions, client.Actions())
	})

	table.DescribeTable("Get API resource list", func(version string) {
		rr := doGetRequest("/apis/upload.cdi.kubevirt.io/" + version)

		resourceList := metav1.APIResourceList{}
		err := json.Unmarshal(rr.Body.Bytes(), &resourceList)
		Expect(err).ToNot(HaveOccurred())

		expectedResourceList := metav1.APIResourceList{
			TypeMeta: metav1.TypeMeta{
				Kind:       "APIResourceList",
				APIVersion: "v1",
			},
			GroupVersion: "upload.cdi.kubevirt.io/" + version,
			APIResources: []metav1.APIResource{
				{
					Name:         "uploadtokenrequests",
					SingularName: "uploadtokenrequest",
					Namespaced:   true,
					Group:        "upload.cdi.kubevirt.io",
					Version:      version,
					Kind:         "UploadTokenRequest",
					Verbs:        []string{"create"},
					ShortNames:   []string{"utr", "utrs"},
				},
			},
		}

		Expect(reflect.DeepEqual(expectedResourceList, resourceList)).To(BeTrue())
	},
		table.Entry("for beta api", "v1beta1"),
		table.Entry("for alpha api", "v1alpha1"),
	)

	It("Get API group", func() {
		rr := doGetRequest("/apis/upload.cdi.kubevirt.io")

		apiGroup := metav1.APIGroup{}
		err := json.Unmarshal(rr.Body.Bytes(), &apiGroup)
		Expect(err).ToNot(HaveOccurred())

		expectedAPIGroup := getExpectedAPIGroup()

		Expect(reflect.DeepEqual(expectedAPIGroup, apiGroup)).To(BeTrue())
	})

	It("Get root paths", func() {
		rr := doGetRequest("/")

		rootPaths := metav1.RootPaths{}
		err := json.Unmarshal(rr.Body.Bytes(), &rootPaths)
		Expect(err).ToNot(HaveOccurred())

		expectedRootPaths := metav1.RootPaths{
			Paths: []string{
				"/apis",
				"/apis/",
				"/apis/upload.cdi.kubevirt.io",
				"/apis/upload.cdi.kubevirt.io/v1alpha1",
				"/apis/upload.cdi.kubevirt.io/v1beta1",
				"/healthz",
			},
		}

		Expect(reflect.DeepEqual(expectedRootPaths, rootPaths)).To(BeTrue())
	})

	It("Get API group list", func() {
		rr := doGetRequest("/apis")

		apiGroupList := metav1.APIGroupList{}
		err := json.Unmarshal(rr.Body.Bytes(), &apiGroupList)
		Expect(err).ToNot(HaveOccurred())

		expectedAPIGroupList := metav1.APIGroupList{
			TypeMeta: metav1.TypeMeta{
				Kind:       "APIGroupList",
				APIVersion: "v1",
			},
			Groups: []metav1.APIGroup{
				getExpectedAPIGroup(),
			},
		}

		Expect(reflect.DeepEqual(expectedAPIGroupList, apiGroupList)).To(BeTrue())
	})

	It("Healthz", func() {
		rr := doGetRequest("/healthz")

		status := rr.Code
		Expect(status).To(Equal(http.StatusOK))
	})

	type args struct {
		authorizer CdiAPIAuthorizer
		pvc        *v1.PersistentVolumeClaim
	}

	signingKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	request := &cdiuploadv1.UploadTokenRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-token",
			Namespace: "default",
		},
		Spec: cdiuploadv1.UploadTokenRequestSpec{
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
		panic(err)
	}

	authorizeSuccess := &testAuthorizer{allowed: true}

	table.DescribeTable("Get token", func(args args, expectedStatus int, checkToken bool) {
		kubeobjects := []runtime.Object{}
		if args.pvc != nil {
			kubeobjects = append(kubeobjects, args.pvc)
		}
		client := k8sfake.NewSimpleClientset(kubeobjects...)

		app := &cdiAPIApp{client: client,
			privateSigningKey: signingKey,
			authorizer:        args.authorizer,
			tokenGenerator:    newUploadTokenGenerator(signingKey)}
		app.composeUploadTokenAPI()

		req, err := http.NewRequest("POST",
			"/apis/upload.cdi.kubevirt.io/v1beta1/namespaces/default/uploadtokenrequests",
			bytes.NewReader(serializedRequest))
		Expect(err).ToNot(HaveOccurred())
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		app.container.ServeHTTP(rr, req)

		status := rr.Code
		Expect(status).To(Equal(expectedStatus))

		if checkToken {
			uploadTokenRequest := &cdiuploadv1.UploadTokenRequest{}
			err := json.Unmarshal(rr.Body.Bytes(), &uploadTokenRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(uploadTokenRequest.Status.Token).To(Not(Equal("")))
		}
	},
		table.Entry("authoriser error",
			args{
				authorizer: &testAuthorizer{allowed: false, reason: "", err: fmt.Errorf("Error")},
			},
			http.StatusInternalServerError,
			false),

		table.Entry("authoriser not allowed",
			args{
				authorizer: &testAuthorizer{allowed: false, reason: "bad person", err: nil},
			},
			http.StatusUnauthorized,
			false),

		table.Entry("pvc does not exist",
			args{
				authorizer: authorizeSuccess,
			},
			http.StatusOK,
			false),

		table.Entry("upload possible",
			args{
				authorizer: authorizeSuccess,
				pvc:        pvc,
			},
			http.StatusOK,
			true),
	)
})
