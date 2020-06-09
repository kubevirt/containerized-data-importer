package main

import (
	"fmt"
	"strings"

	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"github.com/go-openapi/spec"
	"k8s.io/kube-openapi/pkg/builder"
	"k8s.io/kube-openapi/pkg/common"

	cdicorev1 "kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1"
	cdiuploadv1 "kubevirt.io/containerized-data-importer/pkg/apis/upload/v1beta1"
)

// code stolen/adapted from https://github.com/kubevirt/kubevirt/blob/master/pkg/util/openapi/openapi.go

func createOpenAPIConfig(webServices []*restful.WebService) restfulspec.Config {
	return restfulspec.Config{
		WebServices:                   webServices,
		WebServicesURL:                "",
		APIPath:                       "/swaggerapi",
		PostBuildSwaggerObjectHandler: addInfoToSwaggerObject,
	}
}

func addInfoToSwaggerObject(swo *spec.Swagger) {
	swo.Info = &spec.Info{
		InfoProps: spec.InfoProps{
			Title:       "KubeVirt Containerized Data Importer API",
			Description: "Containerized Data Importer for KubeVirt.",
			Contact: &spec.ContactInfo{
				Name:  "kubevirt-dev",
				Email: "kubevirt-dev@googlegroups.com",
				URL:   "https://github.com/kubevirt/containerized-data-importer",
			},
			License: &spec.License{
				Name: "Apache 2.0",
				URL:  "https://www.apache.org/licenses/LICENSE-2.0",
			},
		},
	}
	swo.SecurityDefinitions = spec.SecurityDefinitions{
		"BearerToken": &spec.SecurityScheme{
			SecuritySchemeProps: spec.SecuritySchemeProps{
				Type:        "apiKey",
				Name:        "authorization",
				In:          "header",
				Description: "Bearer Token authentication",
			},
		},
	}
	swo.Security = make([]map[string][]string, 1)
	swo.Security[0] = map[string][]string{"BearerToken": {}}
}

func createConfig() *common.Config {
	return &common.Config{
		CommonResponses: map[int]spec.Response{
			401: {
				ResponseProps: spec.ResponseProps{
					Description: "Unauthorized",
				},
			},
		},
		Info: &spec.Info{
			InfoProps: spec.InfoProps{
				Title:       "KubeVirt Containerized Data Importer API",
				Description: "Containerized Data Importer for KubeVirt.",
				Contact: &spec.ContactInfo{
					Name:  "kubevirt-dev",
					Email: "kubevirt-dev@googlegroups.com",
					URL:   "https://github.com/kubevirt/containerized-data-importer",
				},
				License: &spec.License{
					Name: "Apache 2.0",
					URL:  "https://www.apache.org/licenses/LICENSE-2.0",
				},
			},
		},
		SecurityDefinitions: &spec.SecurityDefinitions{
			"BearerToken": &spec.SecurityScheme{
				SecuritySchemeProps: spec.SecuritySchemeProps{
					Type:        "apiKey",
					Name:        "authorization",
					In:          "header",
					Description: "Bearer Token authentication",
				},
			},
		},
		GetDefinitions: func(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
			m := cdicorev1.GetOpenAPIDefinitions(ref)
			m2 := cdiuploadv1.GetOpenAPIDefinitions(ref)
			for k, v := range m2 {
				if _, ok := m[k]; !ok {
					m[k] = v
				}
			}
			return m
		},
	}
}

func loadOpenAPISpec(webServices []*restful.WebService) *spec.Swagger {
	config := createConfig()
	openapispec, err := builder.BuildOpenAPISpec(webServices, config)
	if err != nil {
		panic(fmt.Errorf("Failed to build swagger: %s", err))
	}

	// creationTimestamp, lastProbeTime and lastTransitionTime are deserialized as "null"
	// Fix it here until
	// https://github.com/kubernetes/kubernetes/issues/66899 is ready
	// Otherwise CRDs can't use templates which contain metadata and controllers
	// can't set conditions without timestamps
	objectMeta, exists := openapispec.Definitions["v1.ObjectMeta"]
	if exists {
		prop := objectMeta.Properties["creationTimestamp"]
		prop.Type = spec.StringOrArray{"string", "null"}
		// mask v1.Time as in validation v1.Time override sting,null type
		prop.Ref = spec.Ref{}
		objectMeta.Properties["creationTimestamp"] = prop
	}

	for k, s := range openapispec.Definitions {
		// allow nullable statuses
		if status, found := s.Properties["status"]; found {
			if !status.Type.Contains("string") {
				definitionName := strings.Split(status.Ref.GetPointer().String(), "/")[2]
				object := openapispec.Definitions[definitionName]
				object.Nullable = true
				openapispec.Definitions[definitionName] = object
			}
		}

		if strings.HasSuffix(k, "Condition") {
			prop := s.Properties["lastProbeTime"]
			prop.Type = spec.StringOrArray{"string", "null"}
			prop.Ref = spec.Ref{}
			s.Properties["lastProbeTime"] = prop

			prop = s.Properties["lastTransitionTime"]
			prop.Type = spec.StringOrArray{"string", "null"}
			prop.Ref = spec.Ref{}
			s.Properties["lastTransitionTime"] = prop
		}
		if k == "v1.HTTPGetAction" {
			prop := s.Properties["port"]
			prop.Type = spec.StringOrArray{"string", "number"}
			// As intstr.IntOrString, the ref for that must be masked
			prop.Ref = spec.Ref{}
			s.Properties["port"] = prop
		}
		if k == "v1.TCPSocketAction" {
			prop := s.Properties["port"]
			prop.Type = spec.StringOrArray{"string", "number"}
			// As intstr.IntOrString, the ref for that must be masked
			prop.Ref = spec.Ref{}
			s.Properties["port"] = prop
		}
		if k == "v1.PersistentVolumeClaimSpec" {
			for i, r := range s.Required {
				if r == "dataSource" {
					s.Required = append(s.Required[:i], s.Required[i+1:]...)
					openapispec.Definitions[k] = s
					break
				}
			}
		}
	}

	return openapispec
}
