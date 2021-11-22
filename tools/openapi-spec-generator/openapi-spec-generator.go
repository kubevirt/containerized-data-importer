/*
Copyright 2018 The CDI Authors.

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

package main

import (
	"encoding/json"
	"fmt"

	"github.com/emicklei/go-restful"
	"k8s.io/kube-openapi/pkg/common"

	cdicorev1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiuploadv1 "kubevirt.io/containerized-data-importer/pkg/apis/upload/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/apiserver"
	"kubevirt.io/containerized-data-importer/pkg/util/openapi"
)

func dumpOpenAPISpec(apiws []*restful.WebService, getDefinitions common.GetOpenAPIDefinitions) {
	openapispec, err := openapi.LoadOpenAPISpec(apiws, getDefinitions)
	if err != nil {
		panic(fmt.Errorf("Failed to build swagger: %s", err))
	}

	data, err := json.MarshalIndent(openapispec, " ", " ")
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	fmt.Println(string(data))
}

func main() {
	webservices := apiserver.UploadTokenRequestAPI()
	webservices = append(webservices, CoreAPI()...)
	dumpOpenAPISpec(webservices, func(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
		m := cdicorev1.GetOpenAPIDefinitions(ref)
		m2 := cdiuploadv1.GetOpenAPIDefinitions(ref)
		for k, v := range m2 {
			if _, ok := m[k]; !ok {
				m[k] = v
			}
		}
		return m
	})
}
