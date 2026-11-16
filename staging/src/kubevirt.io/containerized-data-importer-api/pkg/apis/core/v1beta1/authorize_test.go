/*
Copyright 2026 The CDI Authors.

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

package v1beta1_test

import (
	"errors"
	"testing"

	authentication "k8s.io/api/authentication/v1"
	authorization "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

type authorizationTestProxy struct {
	lookups                     int
	allowed                     bool
	namespaceErr, dataSourceErr error
	reviews                     []*authorization.SubjectAccessReview
}

func (p *authorizationTestProxy) CreateSar(sar *authorization.SubjectAccessReview) (*authorization.SubjectAccessReview, error) {
	p.reviews = append(p.reviews, sar)
	sar.Status.Allowed = p.allowed
	return sar, nil
}

func (p *authorizationTestProxy) GetNamespace(name string) (*corev1.Namespace, error) {
	p.lookups++
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}, p.namespaceErr
}

func (p *authorizationTestProxy) GetDataSource(namespace, name string) (*cdiv1.DataSource, error) {
	p.lookups++
	return &cdiv1.DataSource{Spec: cdiv1.DataSourceSpec{Source: cdiv1.DataSourceSource{PVC: &cdiv1.DataVolumeSourcePVC{Namespace: "source", Name: "disk"}}}}, p.dataSourceErr
}

func TestAuthorizeClone(t *testing.T) {
	namespaceNotFound := k8serrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, "source")
	dataSourceNotFound := k8serrors.NewNotFound(schema.GroupResource{Group: cdiv1.SchemeGroupVersion.Group, Resource: "datasources"}, "source")

	tests := []struct {
		name, annotation, scenario string
		user, expectedAllowed      bool
		expectedErr                error
	}{
		{"service account without annotations: denied", "", "denied", false, false, nil},
		{"service account without annotations: allowed", "", "allowed", false, true, nil},
		{"service account without annotations: missing namespace", "", "missing namespace", false, false, namespaceNotFound},
		{"service account without annotations: missing datasource", "", "missing datasource", false, false, dataSourceNotFound},
		{"service account without annotations: same namespace", "", "same namespace", false, true, nil},
		{"service account with checkStaticVolume: denied", cdiv1.AnnCheckStaticVolume, "denied", false, false, nil},
		{"service account with checkStaticVolume: allowed", cdiv1.AnnCheckStaticVolume, "allowed", false, true, nil},
		{"service account with checkStaticVolume: missing namespace", cdiv1.AnnCheckStaticVolume, "missing namespace", false, false, namespaceNotFound},
		{"service account with checkStaticVolume: missing datasource", cdiv1.AnnCheckStaticVolume, "missing datasource", false, false, dataSourceNotFound},
		{"service account with checkStaticVolume: same namespace", cdiv1.AnnCheckStaticVolume, "same namespace", false, true, nil},
		{"service account with prePopulated: denied", cdiv1.AnnPrePopulated, "denied", false, true, cdiv1.ErrNoTokenOkay},
		{"service account with prePopulated: allowed", cdiv1.AnnPrePopulated, "allowed", false, true, nil},
		{"service account with prePopulated: missing namespace", cdiv1.AnnPrePopulated, "missing namespace", false, true, cdiv1.ErrNoTokenOkay},
		{"service account with prePopulated: missing datasource", cdiv1.AnnPrePopulated, "missing datasource", false, true, cdiv1.ErrNoTokenOkay},
		{"service account with prePopulated: same namespace", cdiv1.AnnPrePopulated, "same namespace", false, true, nil},
		{"user without annotations: denied", "", "denied", true, false, nil},
		{"user without annotations: allowed", "", "allowed", true, true, nil},
		{"user without annotations: missing namespace", "", "missing namespace", true, false, namespaceNotFound},
		{"user without annotations: missing datasource", "", "missing datasource", true, false, dataSourceNotFound},
		{"user without annotations: same namespace", "", "same namespace", true, true, nil},
		{"user with checkStaticVolume: denied", cdiv1.AnnCheckStaticVolume, "denied", true, true, cdiv1.ErrNoTokenOkay},
		{"user with checkStaticVolume: allowed", cdiv1.AnnCheckStaticVolume, "allowed", true, true, nil},
		{"user with checkStaticVolume: missing namespace", cdiv1.AnnCheckStaticVolume, "missing namespace", true, true, cdiv1.ErrNoTokenOkay},
		{"user with checkStaticVolume: missing datasource", cdiv1.AnnCheckStaticVolume, "missing datasource", true, true, cdiv1.ErrNoTokenOkay},
		{"user with checkStaticVolume: same namespace", cdiv1.AnnCheckStaticVolume, "same namespace", true, true, nil},
		{"user with prePopulated: denied", cdiv1.AnnPrePopulated, "denied", true, true, cdiv1.ErrNoTokenOkay},
		{"user with prePopulated: allowed", cdiv1.AnnPrePopulated, "allowed", true, true, nil},
		{"user with prePopulated: missing namespace", cdiv1.AnnPrePopulated, "missing namespace", true, true, cdiv1.ErrNoTokenOkay},
		{"user with prePopulated: missing datasource", cdiv1.AnnPrePopulated, "missing datasource", true, true, cdiv1.ErrNoTokenOkay},
		{"user with prePopulated: same namespace", cdiv1.AnnPrePopulated, "same namespace", true, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dv := &cdiv1.DataVolume{
				ObjectMeta: metav1.ObjectMeta{Name: "target", Namespace: "target"},
				Spec: cdiv1.DataVolumeSpec{
					Source: &cdiv1.DataVolumeSource{PVC: &cdiv1.DataVolumeSourcePVC{Namespace: "source", Name: "disk"}},
				},
			}
			if tt.annotation != "" {
				dv.Annotations = map[string]string{tt.annotation: "true"}
			}
			proxy := &authorizationTestProxy{allowed: tt.scenario == "allowed"}
			switch tt.scenario {
			case "missing namespace":
				proxy.namespaceErr = namespaceNotFound
			case "missing datasource":
				dv.Spec.Source = nil
				dv.Spec.SourceRef = &cdiv1.DataVolumeSourceRef{Kind: cdiv1.DataVolumeDataSource, Name: "source"}
				proxy.dataSourceErr = dataSourceNotFound
			case "same namespace":
				dv.Spec.Source.PVC.Namespace = "target"
			}
			var response cdiv1.CloneAuthResponse
			var err error
			if tt.user {
				response, err = dv.AuthorizeUser("target", "target", proxy, authentication.UserInfo{Username: "tenant"})
			} else {
				response, err = dv.AuthorizeSA("target", "target", proxy, "target", "default")
			}
			if response.Allowed != tt.expectedAllowed {
				t.Errorf("Allowed = %v, want %v", response.Allowed, tt.expectedAllowed)
			}
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("error = %v, want %v", err, tt.expectedErr)
			}
			if !tt.expectedAllowed && tt.expectedErr == nil && response.Reason == "" {
				t.Error("expected a reason for the SAR denial")
			}
			if tt.scenario == "denied" || tt.scenario == "allowed" {
				if len(proxy.reviews) == 0 {
					t.Fatal("expected a SubjectAccessReview")
				}
				expectedUser := "system:serviceaccount:target:default"
				if tt.user {
					expectedUser = "tenant"
				}
				for _, sar := range proxy.reviews {
					if sar.Spec.User != expectedUser {
						t.Errorf("SAR user = %q, want %q", sar.Spec.User, expectedUser)
					}
					if sar.Spec.ResourceAttributes == nil || sar.Spec.ResourceAttributes.Namespace != "source" || sar.Spec.ResourceAttributes.Name != "disk" {
						t.Errorf("unexpected SAR resource attributes: %+v", sar.Spec.ResourceAttributes)
					}
				}
			} else if len(proxy.reviews) != 0 {
				t.Errorf("unexpected SubjectAccessReviews: %v", proxy.reviews)
			}
		})
	}
}

func TestAuthorizeSANonClone(t *testing.T) {
	tests := []struct {
		name   string
		source *cdiv1.DataVolumeSource
	}{
		{"HTTP import", &cdiv1.DataVolumeSource{HTTP: &cdiv1.DataVolumeSourceHTTP{URL: "https://example.com/disk"}}},
		{"registry import", &cdiv1.DataVolumeSource{Registry: &cdiv1.DataVolumeSourceRegistry{}}},
		{"upload", &cdiv1.DataVolumeSource{Upload: &cdiv1.DataVolumeSourceUpload{}}},
		{"blank", &cdiv1.DataVolumeSource{Blank: &cdiv1.DataVolumeBlankImage{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dv := &cdiv1.DataVolume{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{cdiv1.AnnCheckStaticVolume: "true"}},
				Spec:       cdiv1.DataVolumeSpec{Source: tt.source},
			}
			proxy := &authorizationTestProxy{}
			response, err := dv.AuthorizeSA("target", "target", proxy, "target", "default")
			if proxy.lookups != 0 || len(proxy.reviews) != 0 {
				t.Error("non-clone source made authorization API calls")
			}
			if !response.Allowed || !errors.Is(err, cdiv1.ErrNoTokenOkay) {
				t.Errorf("expected non-clone token waiver, got %+v, %v", response, err)
			}
		})
	}
}
