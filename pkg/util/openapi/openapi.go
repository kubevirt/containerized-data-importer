package openapi

import (
	"github.com/emicklei/go-restful"
	"k8s.io/kube-openapi/pkg/builder"
	"k8s.io/kube-openapi/pkg/common"
	k8sspec "k8s.io/kube-openapi/pkg/validation/spec"
)

// code stolen/adapted from https://github.com/kubevirt/kubevirt/blob/main/pkg/util/openapi/openapi.go

func createConfig(getDefinitions common.GetOpenAPIDefinitions) *common.Config {
	return &common.Config{
		CommonResponses: map[int]k8sspec.Response{
			401: {
				ResponseProps: k8sspec.ResponseProps{
					Description: "Unauthorized",
				},
			},
		},
		Info: &k8sspec.Info{
			InfoProps: k8sspec.InfoProps{
				Title:       "KubeVirt Containerized Data Importer API",
				Description: "Containerized Data Importer for KubeVirt.",
				Contact: &k8sspec.ContactInfo{
					Name:  "kubevirt-dev",
					Email: "kubevirt-dev@googlegroups.com",
					URL:   "https://github.com/kubevirt/containerized-data-importer",
				},
				License: &k8sspec.License{
					Name: "Apache 2.0",
					URL:  "https://www.apache.org/licenses/LICENSE-2.0",
				},
			},
		},
		SecurityDefinitions: &k8sspec.SecurityDefinitions{
			"BearerToken": &k8sspec.SecurityScheme{
				SecuritySchemeProps: k8sspec.SecuritySchemeProps{
					Type:        "apiKey",
					Name:        "authorization",
					In:          "header",
					Description: "Bearer Token authentication",
				},
			},
		},
		GetDefinitions: getDefinitions,
	}
}

// LoadOpenAPISpec creates a swagger doc for given webservice(s)
func LoadOpenAPISpec(webServices []*restful.WebService, getDefinitions common.GetOpenAPIDefinitions) (*k8sspec.Swagger, error) {
	config := createConfig(getDefinitions)
	return builder.BuildOpenAPISpec(webServices, config)
}
