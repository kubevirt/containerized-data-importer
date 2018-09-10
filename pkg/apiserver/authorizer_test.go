package apiserver

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"testing"

	"github.com/emicklei/go-restful"

	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
)

func fakeRequest() *restful.Request {
	req := &restful.Request{}
	req.Request = &http.Request{}
	req.Request.URL = &url.URL{}
	req.Request.Header = make(map[string][]string)
	req.Request.Header[userHeader] = []string{"user"}
	req.Request.Header[groupHeader] = []string{"userGroup"}
	req.Request.Header[userExtraHeaderPrefix+"test"] = []string{"userExtraValue"}
	req.Request.URL.Path = "/apis/upload.cdi.kubevirt.io/v1alpha1/namespaces/default/uploadtokenrequest"
	return req
}

func newAuthorizor() authorizor {

	app := authorizor{}
	app.userHeaders = append(app.userHeaders, userHeader)
	app.groupHeaders = append(app.groupHeaders, groupHeader)
	app.userExtraHeaderPrefixes = append(app.userExtraHeaderPrefixes, userExtraHeaderPrefix)
	return app
}

func TestRejectUnauthenticatedUser(t *testing.T) {

	kubeobjects := []runtime.Object{}
	client := k8sfake.NewSimpleClientset(kubeobjects...)
	authClient := client.AuthorizationV1beta1()

	app := newAuthorizor()
	app.subjectAccessReview = authClient.SubjectAccessReviews()

	allowed, reason, err := app.Authorize(fakeRequest())

	if allowed != false {
		t.Errorf("Should not have allowed request")
	} else if reason != "request is not authenticated" {
		t.Errorf("Unexpected reason")
	} else if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestRejectUnauthorizedUser(t *testing.T) {
	fakecert := &x509.Certificate{}
	kubeobjects := []runtime.Object{}
	client := k8sfake.NewSimpleClientset(kubeobjects...)
	authClient := client.AuthorizationV1beta1()

	app := newAuthorizor()
	app.subjectAccessReview = authClient.SubjectAccessReviews()

	// Add fake client cert info to prove authentication
	req := fakeRequest()
	req.Request.TLS = &tls.ConnectionState{}
	req.Request.TLS.PeerCertificates = append(req.Request.TLS.PeerCertificates, fakecert)

	allowed, reason, err := app.Authorize(req)

	if allowed != false {
		t.Errorf("Should not have allowed request")
	} else if reason != "" {
		// reason is empty because the fake client is going to return a status
		// section without a reason set
		t.Errorf("Unexpected reason %s", reason)
	} else if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestGenerateAccessReview(t *testing.T) {
	app := newAuthorizor()
	req := fakeRequest()
	authReview, err := app.generateAccessReview(req)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if authReview == nil {
		t.Errorf("Authreview not generated: %v", err)
	}
}

func TestGenerateAccessReviewPathErrGroup(t *testing.T) {
	app := newAuthorizor()
	req := fakeRequest()
	req.Request.URL.Path = "/apis/NOTOURGROUP/v1alpha1/namespaces/default/uploadtokenrequest"
	authReview, err := app.generateAccessReview(req)

	if err == nil {
		t.Errorf("expected an error")
	} else if authReview != nil {
		t.Errorf("Authreview should not have been generated")
	}
}

func TestGenerateAccessReviewPathErrResource(t *testing.T) {
	app := newAuthorizor()
	req := fakeRequest()
	req.Request.URL.Path = "/apis/upload.cdi.kubevirt.io/v1alpha1/namespaces/default/NOTOURRESOURCE"
	authReview, err := app.generateAccessReview(req)

	if err == nil {
		t.Errorf("expected an error")
	} else if authReview != nil {
		t.Errorf("Authreview should not have been generated")
	}
}

func TestAccessReviewSuccess(t *testing.T) {
	app := newAuthorizor()
	req := fakeRequest()
	authReview, err := app.generateAccessReview(req)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if authReview == nil {
		t.Errorf("Authreview not generated: %v", err)
	}

	authReview.Status.Allowed = true

	allowed, reason := isAllowed(authReview)

	if allowed != true {
		t.Errorf("Auth review should have been allowed")
	} else if reason != "" {
		t.Errorf("reason should have been empty, got %s", reason)
	}
}

func TestAccessReviewNotAllowed(t *testing.T) {
	app := newAuthorizor()
	req := fakeRequest()
	authReview, err := app.generateAccessReview(req)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	} else if authReview == nil {
		t.Errorf("Authreview not generated: %v", err)
	}

	authReview.Status.Allowed = false
	authReview.Status.Reason = "because"

	allowed, reason := isAllowed(authReview)

	if allowed == true {
		t.Errorf("Auth review should not have been allowed")
	} else if reason != "because" {
		t.Errorf("reason should not have been empty")
	}
}
