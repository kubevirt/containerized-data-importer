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
	"crypto/x509"
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/emicklei/go-restful"

	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func fakeRequest() *restful.Request {
	req := &restful.Request{}
	req.Request = &http.Request{}
	req.Request.Method = "POST"
	req.Request.URL = &url.URL{}
	req.Request.Header = make(map[string][]string)
	req.Request.Header[userHeader] = []string{"user"}
	req.Request.Header[groupHeader] = []string{"userGroup"}
	req.Request.Header[userExtraHeaderPrefix+"test"] = []string{"userExtraValue"}
	req.Request.URL.Path = "/apis/upload.cdi.kubevirt.io/v1beta1/namespaces/default/uploadtokenrequests"
	return req
}

func newAuthorizor() *authorizor {
	authConfig := &AuthConfig{
		UserHeaders:        []string{userHeader},
		GroupHeaders:       []string{groupHeader},
		ExtraPrefixHeaders: []string{userExtraHeaderPrefix},
	}
	authConfigWatcher := &authConfigWatcher{config: authConfig}
	app := authorizor{authConfigWatcher: authConfigWatcher}
	return &app
}

var _ = Describe("Authorizer test", func() {
	It("Reject unauthenticated user", func() {
		client := k8sfake.NewSimpleClientset()
		authClient := client.AuthorizationV1beta1()

		app := newAuthorizor()
		app.subjectAccessReview = authClient.SubjectAccessReviews()

		allowed, reason, err := app.Authorize(fakeRequest())
		Expect(allowed).To(BeFalse())
		Expect(reason).To(Equal("request is not authenticated"))
		Expect(err).ToNot(HaveOccurred())
	})

	It("Reject unauthorized user", func() {
		fakecert, fakecert2 := &x509.Certificate{}, &x509.Certificate{}
		client := k8sfake.NewSimpleClientset()
		authClient := client.AuthorizationV1beta1()

		app := newAuthorizor()
		app.subjectAccessReview = authClient.SubjectAccessReviews()

		// Add fake client cert info to prove authentication
		req := fakeRequest()
		req.Request.TLS = &tls.ConnectionState{}
		req.Request.TLS.PeerCertificates = append(req.Request.TLS.PeerCertificates, fakecert)
		req.Request.TLS.VerifiedChains = append(req.Request.TLS.VerifiedChains, []*x509.Certificate{fakecert, fakecert2})

		allowed, reason, err := app.Authorize(req)
		Expect(allowed).To(BeFalse())

		// reason is empty because the fake client is going to return a status
		// section without a reason set
		Expect(reason).To(Equal(""))

		Expect(err).ToNot(HaveOccurred())
	})

	It("Generate access review", func() {
		app := newAuthorizor()
		req := fakeRequest()
		authReview, err := app.generateAccessReview(req)
		Expect(err).ToNot(HaveOccurred())
		Expect(authReview).ToNot(BeNil())
	})

	It("Generate access review path err group", func() {
		app := newAuthorizor()
		req := fakeRequest()
		req.Request.URL.Path = "/apis/NOTOURGROUP/v1alpha1/namespaces/default/uploadtokenrequests"
		authReview, err := app.generateAccessReview(req)
		Expect(err).To(HaveOccurred())
		Expect(authReview).To(BeNil())
	})

	It("Generate access review path err resource", func() {
		app := newAuthorizor()
		req := fakeRequest()
		req.Request.URL.Path = "/apis/upload.cdi.kubevirt.io/v1beta1/namespaces/default/NOTOURRESOURCE"
		authReview, err := app.generateAccessReview(req)
		Expect(err).To(HaveOccurred())
		Expect(authReview).To(BeNil())
	})

	It("Access review success", func() {
		app := newAuthorizor()
		req := fakeRequest()
		authReview, err := app.generateAccessReview(req)
		Expect(err).ToNot(HaveOccurred())
		Expect(authReview).ToNot(BeNil())

		authReview.Status.Allowed = true

		allowed, reason := isAllowed(authReview)
		Expect(allowed).To(BeTrue())
		Expect(reason).To(Equal(""))
	})

	It("Access review not allowed", func() {
		app := newAuthorizor()
		req := fakeRequest()
		authReview, err := app.generateAccessReview(req)
		Expect(err).ToNot(HaveOccurred())
		Expect(authReview).ToNot(BeNil())

		authReview.Status.Allowed = false
		authReview.Status.Reason = "because"

		allowed, reason := isAllowed(authReview)
		Expect(allowed).To(BeFalse())
		Expect(reason).To(Equal("because"))
	})
})
