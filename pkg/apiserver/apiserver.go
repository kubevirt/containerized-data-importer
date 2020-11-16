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
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	restful "github.com/emicklei/go-restful"
	"github.com/go-openapi/spec"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	cdiuploadv1 "kubevirt.io/containerized-data-importer/pkg/apis/upload/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/apiserver/webhooks"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/keys"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/openapi"
)

const (
	// selfsigned cert secret name
	apiSigningKeySecretName = "cdi-api-signing-key"

	uploadTokenGroup = "upload.cdi.kubevirt.io"

	dvValidatePath = "/datavolume-validate"

	dvMutatePath = "/datavolume-mutate"

	cdiValidatePath = "/cdi-validate"

	healthzPath = "/healthz"
)

var uploadTokenVersions = []string{"v1beta1", "v1alpha1"}

// CdiAPIServer is the public interface to the CDI API
type CdiAPIServer interface {
	Start(<-chan struct{}) error
}

// CertWatcher is the interface for resources that watch certs
type CertWatcher interface {
	GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error)
}

type cdiAPIApp struct {
	bindAddress string
	bindPort    uint

	client           kubernetes.Interface
	aggregatorClient aggregatorclient.Interface
	cdiClient        cdiclient.Interface

	privateSigningKey *rsa.PrivateKey

	container *restful.Container

	authorizer        CdiAPIAuthorizer
	authConfigWatcher AuthConfigWatcher

	certWarcher CertWatcher

	tokenGenerator token.Generator
}

// UploadTokenRequestAPI returns web service for swagger generation
func UploadTokenRequestAPI() []*restful.WebService {
	app := cdiAPIApp{}
	app.composeUploadTokenAPI()
	return app.container.RegisteredWebServices()
}

// NewCdiAPIServer returns an initialized CDI api server
func NewCdiAPIServer(bindAddress string,
	bindPort uint,
	client kubernetes.Interface,
	aggregatorClient aggregatorclient.Interface,
	cdiClient cdiclient.Interface,
	authorizor CdiAPIAuthorizer,
	authConfigWatcher AuthConfigWatcher,
	certWatcher CertWatcher) (CdiAPIServer, error) {
	var err error
	app := &cdiAPIApp{
		bindAddress:       bindAddress,
		bindPort:          bindPort,
		client:            client,
		aggregatorClient:  aggregatorClient,
		cdiClient:         cdiClient,
		authorizer:        authorizor,
		authConfigWatcher: authConfigWatcher,
		certWarcher:       certWatcher,
	}

	err = app.getKeysAndCerts()
	if err != nil {
		return nil, errors.Errorf("Unable to get self signed cert: %v\n", errors.WithStack(err))
	}

	app.composeUploadTokenAPI()

	app.container.Filter(func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		var username = "-"
		if req.Request.URL.User != nil {
			if name := req.Request.URL.User.Username(); name != "" {
				username = name
			}
		}
		chain.ProcessFilter(req, resp)

		klog.V(3).Infof("----------------------------")
		klog.V(3).Infof("remoteAddress:%s", strings.Split(req.Request.RemoteAddr, ":")[0])
		klog.V(3).Infof("username: %s", username)
		klog.V(3).Infof("method: %s", req.Request.Method)
		klog.V(3).Infof("url: %s", req.Request.URL.RequestURI())
		klog.V(3).Infof("proto: %s", req.Request.Proto)
		klog.V(3).Infof("headers: %v", req.Request.Header)
		klog.V(3).Infof("statusCode: %d", resp.StatusCode())
		klog.V(3).Infof("contentLength: %d", resp.ContentLength())

	})

	err = app.createDataVolumeValidatingWebhook()
	if err != nil {
		return nil, errors.Errorf("failed to create DataVolume validating webhook: %s", err)
	}

	err = app.createDataVolumeMutatingWebhook()
	if err != nil {
		return nil, errors.Errorf("failed to create DataVolume mutating webhook: %s", err)
	}

	err = app.createCDIValidatingWebhook()
	if err != nil {
		return nil, errors.Errorf("failed to create CDI validating webhook: %s", err)
	}

	return app, nil
}

func newUploadTokenGenerator(key *rsa.PrivateKey) token.Generator {
	return token.NewGenerator(common.UploadTokenIssuer, key, 5*time.Minute)
}

func (app *cdiAPIApp) Start(ch <-chan struct{}) error {
	return app.startTLS(ch)
}

func (app *cdiAPIApp) getKeysAndCerts() error {
	namespace := util.GetNamespace()

	privateKey, err := keys.GetOrCreatePrivateKey(app.client, namespace, apiSigningKeySecretName)
	if err != nil {
		return errors.Wrap(err, "Error getting/creating signing key")
	}

	app.privateSigningKey = privateKey

	app.tokenGenerator = newUploadTokenGenerator(privateKey)

	return nil
}

func (app *cdiAPIApp) getTLSConfig() (*tls.Config, error) {
	authConfig := app.authConfigWatcher.GetAuthConfig()

	cert, err := app.certWarcher.GetCertificate(nil)
	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		ClientCAs:    authConfig.CertPool,
		ClientAuth:   tls.VerifyClientCertIfGiven,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			if len(verifiedChains) == 0 {
				return nil
			}
			for i := range verifiedChains {
				if authConfig.ValidateName(verifiedChains[i][0].Subject.CommonName) {
					return nil
				}
			}
			return fmt.Errorf("no valid subject specified")
		},
	}
	tlsConfig.BuildNameToCertificate()

	return tlsConfig, nil
}

func (app *cdiAPIApp) startTLS(stopChan <-chan struct{}) error {
	errChan := make(chan error)

	tlsConfig, err := app.getTLSConfig()
	if err != nil {
		return err
	}

	tlsConfig.GetConfigForClient = func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
		klog.V(3).Info("Getting TLS config")
		config, err := app.getTLSConfig()
		if err != nil {
			klog.Errorf("Error %+v getting TLS config", err)
		}
		return config, err
	}

	server := &http.Server{
		Addr:      fmt.Sprintf("%s:%d", app.bindAddress, app.bindPort),
		TLSConfig: tlsConfig,
		Handler:   app.container,
	}

	go func() {
		errChan <- server.ListenAndServeTLS("", "")
	}()

	select {
	case <-stopChan:
		return server.Shutdown(context.Background())
	case err = <-errChan:
		return err
	}
}

func (app *cdiAPIApp) uploadHandler(request *restful.Request, response *restful.Response) {
	allowed, reason, err := app.authorizer.Authorize(request)

	if err != nil {
		klog.Error(err)
		response.WriteHeader(http.StatusInternalServerError)
		return
	} else if !allowed {
		klog.Infof("Rejected Request: %s", reason)
		response.WriteErrorString(http.StatusUnauthorized, reason)
		return
	}

	namespace := request.PathParameter("namespace")
	defer request.Request.Body.Close()
	body, err := ioutil.ReadAll(request.Request.Body)
	if err != nil {
		klog.Error(err)
		response.WriteError(http.StatusBadRequest, err)
		return
	}

	uploadToken := &cdiuploadv1.UploadTokenRequest{}
	err = json.Unmarshal(body, uploadToken)
	if err != nil {
		klog.Error(err)
		response.WriteError(http.StatusBadRequest, err)
		return
	}

	tokenData := &token.Payload{
		Operation: token.OperationUpload,
		Name:      uploadToken.Spec.PvcName,
		Namespace: namespace,
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "persistentvolumeclaims",
		},
	}

	token, err := app.tokenGenerator.Generate(tokenData)
	if err != nil {
		klog.Error(err)
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	uploadToken.Status.Token = token
	response.WriteAsJson(uploadToken)

}

func uploadTokenAPIGroup() metav1.APIGroup {
	apiGroup := metav1.APIGroup{
		Name: uploadTokenGroup,
		PreferredVersion: metav1.GroupVersionForDiscovery{
			GroupVersion: uploadTokenGroup + "/" + uploadTokenVersions[0],
			Version:      uploadTokenVersions[0],
		},
	}
	for _, v := range uploadTokenVersions {
		apiGroup.Versions = append(apiGroup.Versions, metav1.GroupVersionForDiscovery{
			GroupVersion: uploadTokenGroup + "/" + v,
			Version:      v,
		})
	}
	apiGroup.ServerAddressByClientCIDRs = append(apiGroup.ServerAddressByClientCIDRs, metav1.ServerAddressByClientCIDR{
		ClientCIDR:    "0.0.0.0/0",
		ServerAddress: "",
	})
	apiGroup.Kind = "APIGroup"
	apiGroup.APIVersion = "v1"
	return apiGroup
}

func (app *cdiAPIApp) composeUploadTokenAPI() {
	var allWebServices []*restful.WebService
	objPointer := &cdiuploadv1.UploadTokenRequest{}
	objExample := reflect.ValueOf(objPointer).Elem().Interface()
	objKind := "UploadTokenRequest"
	resource := "uploadtokenrequests"

	groupPath := fmt.Sprintf("/apis/%s", uploadTokenGroup)
	createPath := fmt.Sprintf("/namespaces/{namespace:[a-z0-9][a-z0-9\\-]*}/%s", resource)

	app.container = restful.NewContainer()

	var resourcePaths []string
	for _, v := range uploadTokenVersions {
		// copy v because of go loop variable/closure weirdness
		uploadTokenVersion := v
		resourcePath := fmt.Sprintf("/apis/%s/%s", uploadTokenGroup, uploadTokenVersion)
		resourcePaths = append(resourcePaths, resourcePath)
		uploadTokenWs := new(restful.WebService)
		allWebServices = append(allWebServices, uploadTokenWs)
		uploadTokenWs.Doc("The CDI Upload API.")
		uploadTokenWs.Path(resourcePath)

		uploadTokenWs.Route(uploadTokenWs.POST(createPath).
			Produces("application/json").
			Consumes("application/json").
			Operation("createNamespaced"+objKind+"-"+v).
			To(app.uploadHandler).Reads(objExample).Writes(objExample).
			Doc("Create an UploadTokenRequest object.").
			Returns(http.StatusOK, "OK", objExample).
			Returns(http.StatusCreated, "Created", objExample).
			Returns(http.StatusAccepted, "Accepted", objExample).
			Returns(http.StatusUnauthorized, "Unauthorized", "").
			Param(uploadTokenWs.PathParameter("namespace", "Object name and auth scope, such as for teams and projects").Required(true)))

		uploadTokenWs.Route(uploadTokenWs.GET("/").
			Produces("application/json").Writes(metav1.APIResourceList{}).
			To(func(request *restful.Request, response *restful.Response) {
				list := &metav1.APIResourceList{}
				list.Kind = "APIResourceList"
				list.APIVersion = "v1" // this is the version of the resource list
				list.GroupVersion = uploadTokenGroup + "/" + uploadTokenVersion
				list.APIResources = append(list.APIResources, metav1.APIResource{
					Name:         "uploadtokenrequests",
					SingularName: "uploadtokenrequest",
					Namespaced:   true,
					Group:        uploadTokenGroup,
					Version:      uploadTokenVersion,
					Kind:         "UploadTokenRequest",
					Verbs:        []string{"create"},
					ShortNames:   []string{"utr", "utrs"},
				})
				response.WriteAsJson(list)
			}).
			Operation("getAPIResources-"+v).
			Doc("Get a CDI API resources").
			Returns(http.StatusOK, "OK", metav1.APIResourceList{}).
			Returns(http.StatusNotFound, "Not Found", ""))

		app.container.Add(uploadTokenWs)
	}

	// hack alert, can only have one version represented so get rid of last one
	allWebServices = allWebServices[0 : len(allWebServices)-1]

	ws := new(restful.WebService)
	allWebServices = append(allWebServices, ws)

	paths := append([]string{"/apis", "/apis/", "/openapi/v2", groupPath, healthzPath}, resourcePaths...)
	sort.Strings(paths)

	ws.Route(ws.GET("/").
		Produces("application/json").Writes(metav1.RootPaths{}).
		To(func(request *restful.Request, response *restful.Response) {
			response.WriteAsJson(&metav1.RootPaths{
				Paths: paths,
			})
		}).
		Operation("getUploadRootPaths").
		Doc("Get a CDI API root paths").
		Returns(http.StatusOK, "OK", metav1.RootPaths{}).
		Returns(http.StatusNotFound, "Not Found", ""))

	// K8s needs the ability to query info about a specific API group
	ws.Route(ws.GET(groupPath).
		Produces("application/json").Writes(metav1.APIGroup{}).
		To(func(request *restful.Request, response *restful.Response) {
			response.WriteAsJson(uploadTokenAPIGroup())
		}).
		Operation("getUploadAPIGroup").
		Doc("Get a CDI API Group").
		Returns(http.StatusOK, "OK", metav1.APIGroup{}).
		Returns(http.StatusNotFound, "Not Found", ""))

	// K8s needs the ability to query the list of API groups this endpoint supports
	ws.Route(ws.GET("apis").
		Produces("application/json").Writes(metav1.APIGroupList{}).
		To(func(request *restful.Request, response *restful.Response) {
			list := &metav1.APIGroupList{}
			list.Kind = "APIGroupList"
			list.APIVersion = "v1"
			list.Groups = append(list.Groups, uploadTokenAPIGroup())
			response.WriteAsJson(list)
		}).
		Operation("getUploadAPIs").
		Doc("Get a CDI API GroupList").
		Returns(http.StatusOK, "OK", metav1.APIGroupList{}).
		Returns(http.StatusNotFound, "Not Found", ""))

	once := sync.Once{}
	var openapispec *spec.Swagger
	ws.Route(ws.GET("openapi/v2").
		Consumes("application/json").
		Produces("application/json").
		To(func(request *restful.Request, response *restful.Response) {
			once.Do(func() {
				var err error
				openapispec, err = openapi.LoadOpenAPISpec(allWebServices, cdiuploadv1.GetOpenAPIDefinitions)
				if err != nil {
					panic(fmt.Errorf("Failed to build swagger: %s", err))
				}
			})
			response.WriteAsJson(openapispec)
		}))

	ws.Route(ws.GET(healthzPath).To(app.healthzHandler))

	app.container.Add(ws)
}

func (app *cdiAPIApp) healthzHandler(req *restful.Request, resp *restful.Response) {
	io.WriteString(resp, "OK")
}

func (app *cdiAPIApp) createDataVolumeValidatingWebhook() error {
	app.container.ServeMux.Handle(dvValidatePath, webhooks.NewDataVolumeValidatingWebhook(app.client))
	return nil
}

func (app *cdiAPIApp) createDataVolumeMutatingWebhook() error {
	app.container.ServeMux.Handle(dvMutatePath, webhooks.NewDataVolumeMutatingWebhook(app.client, app.privateSigningKey))
	return nil
}

func (app *cdiAPIApp) createCDIValidatingWebhook() error {
	app.container.ServeMux.Handle(cdiValidatePath, webhooks.NewCDIValidatingWebhook(app.cdiClient))
	return nil
}
