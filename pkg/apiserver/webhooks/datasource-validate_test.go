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
 * Copyright the CDI Authors.
 *
 */

package webhooks

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfield "k8s.io/apimachinery/pkg/util/validation/field"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const dsName = "test-dataSource"

var _ = Describe("Validating DataSource Webhook", func() {

	Context("with DataSource admission review", func() {
		It("should reject review for self-pointing DataSource", func() {
			ds := newDataSource()
			ds.Spec.Source.DataSource = &cdiv1.DataSourceRefSourceDataSource{
				Name:      ds.Name,
				Namespace: ds.Namespace,
			}
			dsBytes, _ := json.Marshal(&ds)
			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.CDIGroupVersionKind.Group,
						Version:  cdiv1.CDIGroupVersionKind.Version,
						Resource: "datasources",
					},
					Object: runtime.RawExtension{
						Raw: dsBytes,
					},
				},
			}
			resp := validateDs(ar)
			Expect(resp.Allowed).To(BeFalse())
			Expect(resp.Result.Details.Causes).To(ConsistOf(metav1.StatusCause{
				Type:    metav1.CauseTypeFieldValueNotSupported,
				Message: "DataSource cannot point to itself",
				Field:   k8sfield.NewPath("DataSource").Child("Spec").Child("Source").Child("DataSource").String(),
			}))
		})
		DescribeTable("should allow", func(name, namespace string) {
			ds := newDataSource()
			ds.Spec.Source.DataSource = &cdiv1.DataSourceRefSourceDataSource{
				Name:      name,
				Namespace: namespace,
			}
			dsBytes, _ := json.Marshal(&ds)
			ar := &admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					Resource: metav1.GroupVersionResource{
						Group:    cdiv1.CDIGroupVersionKind.Group,
						Version:  cdiv1.CDIGroupVersionKind.Version,
						Resource: "datasources",
					},
					Object: runtime.RawExtension{
						Raw: dsBytes,
					},
				},
			}
			resp := validateDs(ar)
			Expect(resp.Allowed).To(BeTrue())
		},
			Entry("DataSource pointing to a DataSource with a different name in a different namespace", "different-name", "different-namespace"),
			Entry("DataSource pointing to a DataSource with the same name in a different namespace", dsName, "different-namespace"),
			Entry("DataSource pointing to a DataSource with a different name in the same namespace", "different-name", metav1.NamespaceDefault),
		)
	})
})

func newDataSource() *cdiv1.DataSource {
	return &cdiv1.DataSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsName,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: cdiv1.DataSourceSpec{
			Source: cdiv1.DataSourceSource{},
		},
	}
}

func validateDs(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	wh := NewDataSourceValidatingWebhook()
	return serve(ar, wh)
}
