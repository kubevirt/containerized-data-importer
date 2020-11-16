package openapi

import (
	"github.com/emicklei/go-restful"
	restfulspec "github.com/emicklei/go-restful-openapi"
	"github.com/go-openapi/spec"
	"k8s.io/kube-openapi/pkg/builder"
	"k8s.io/kube-openapi/pkg/common"
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
	swo.Swagger = "2.0"
	swo.Security = make([]map[string][]string, 1)
	swo.Security[0] = map[string][]string{"BearerToken": {}}
}

func createConfig(getDefinitions common.GetOpenAPIDefinitions) *common.Config {
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
		GetDefinitions: getDefinitions,
	}
}

// LoadOpenAPISpec creates a swagger doc for given webservice(s)
func LoadOpenAPISpec(webServices []*restful.WebService, getDefinitions common.GetOpenAPIDefinitions) (*spec.Swagger, error) {
	config := createConfig(getDefinitions)
	return builder.BuildOpenAPISpec(webServices, config)
}
