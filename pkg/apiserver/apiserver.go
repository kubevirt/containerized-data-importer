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
	"strings"
	"time"

	restful "github.com/emicklei/go-restful"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	cdiuploadv1alpha1 "kubevirt.io/containerized-data-importer/pkg/apis/upload/v1alpha1"
	"kubevirt.io/containerized-data-importer/pkg/apiserver/webhooks"
	cdiclient "kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/pkg/keys"
	"kubevirt.io/containerized-data-importer/pkg/token"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	// selfsigned cert secret name
	apiSigningKeySecretName = "cdi-api-signing-key"

	uploadTokenGroup   = "upload.cdi.kubevirt.io"
	uploadTokenVersion = "v1alpha1"

	dvValidatePath = "/datavolume-validate"

	dvMutatePath = "/datavolume-mutate"

	cdiValidatePath = "/cdi-validate"

	healthzPath = "/healthz"
)

// CdiAPIServer is the public interface to the CDI API
type CdiAPIServer interface {
	Start(<-chan struct{}) error
}

// CertWatcher is the interface for resources that watch certs
type CertWatcher interface {
	GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error)
}

type uploadPossibleFunc func(*v1.PersistentVolumeClaim) error

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

	// test hook
	uploadPossible uploadPossibleFunc
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
		uploadPossible:    controller.UploadPossibleForPVC,
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

	validName := func(name string) bool {
		for _, n := range authConfig.AllowedNames {
			if n == name {
				return true
			}
		}
		return false
	}

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
				if validName(verifiedChains[i][0].Subject.CommonName) {
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

	uploadToken := &cdiuploadv1alpha1.UploadTokenRequest{}
	err = json.Unmarshal(body, uploadToken)
	if err != nil {
		klog.Error(err)
		response.WriteError(http.StatusBadRequest, err)
		return
	}

	pvcName := uploadToken.Spec.PvcName
	pvc, err := app.client.CoreV1().PersistentVolumeClaims(namespace).Get(pvcName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Infof("Rejecting request for PVC %s that doesn't exist", pvcName)
			response.WriteError(http.StatusBadRequest, err)
			return
		}
		klog.Error(err)
		response.WriteError(http.StatusInternalServerError, err)
		return
	}

	if err = app.uploadPossible(pvc); err != nil {
		response.WriteError(http.StatusServiceUnavailable, err)
		return
	}

	tokenData := &token.Payload{
		Operation: token.OperationUpload,
		// this is service or pod but not pvc?
		Name:      pvcName,
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
			GroupVersion: uploadTokenGroup + "/" + uploadTokenVersion,
			Version:      uploadTokenVersion,
		},
	}
	apiGroup.Versions = append(apiGroup.Versions, metav1.GroupVersionForDiscovery{
		GroupVersion: uploadTokenGroup + "/" + uploadTokenVersion,
		Version:      uploadTokenVersion,
	})
	apiGroup.ServerAddressByClientCIDRs = append(apiGroup.ServerAddressByClientCIDRs, metav1.ServerAddressByClientCIDR{
		ClientCIDR:    "0.0.0.0/0",
		ServerAddress: "",
	})
	apiGroup.Kind = "APIGroup"
	apiGroup.APIVersion = "v1"
	return apiGroup
}

func (app *cdiAPIApp) composeUploadTokenAPI() {
	objPointer := &cdiuploadv1alpha1.UploadTokenRequest{}
	objExample := reflect.ValueOf(objPointer).Elem().Interface()
	objKind := "UploadTokenRequest"
	resource := "uploadtokenrequests"

	groupPath := fmt.Sprintf("/apis/%s", uploadTokenGroup)
	resourcePath := fmt.Sprintf("/apis/%s/%s", uploadTokenGroup, uploadTokenVersion)
	createPath := fmt.Sprintf("/namespaces/{namespace:[a-z0-9][a-z0-9\\-]*}/%s", resource)

	app.container = restful.NewContainer()

	uploadTokenWs := new(restful.WebService)
	uploadTokenWs.Doc("The CDI Upload API.")
	uploadTokenWs.Path(resourcePath)

	uploadTokenWs.Route(uploadTokenWs.POST(createPath).
		Produces("application/json").
		Consumes("application/json").
		Operation("createNamespaced"+objKind).
		To(app.uploadHandler).Reads(objExample).Writes(objExample).
		Doc("Create an UploadTokenRequest object.").
		Returns(http.StatusOK, "OK", objExample).
		Returns(http.StatusCreated, "Created", objExample).
		Returns(http.StatusAccepted, "Accepted", objExample).
		Returns(http.StatusUnauthorized, "Unauthorized", nil).
		Param(uploadTokenWs.PathParameter("namespace", "Object name and auth scope, such as for teams and projects").Required(true)))

	// Return empty api resource list.
	// K8s expects to be able to retrieve a resource list for each aggregated
	// app in order to discover what resources it provides. Without returning
	// an empty list here, there's a bug in the k8s resource discovery that
	// breaks kubectl's ability to reference short names for resources.
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
		Operation("getAPIResources").
		Doc("Get a CDI API resources").
		Returns(http.StatusOK, "OK", metav1.APIResourceList{}).
		Returns(http.StatusNotFound, "Not Found", nil))

	app.container.Add(uploadTokenWs)

	ws := new(restful.WebService)

	ws.Route(ws.GET("/").
		Produces("application/json").Writes(metav1.RootPaths{}).
		To(func(request *restful.Request, response *restful.Response) {
			response.WriteAsJson(&metav1.RootPaths{
				Paths: []string{
					"/apis",
					"/apis/",
					groupPath,
					resourcePath,
					healthzPath,
				},
			})
		}).
		Operation("getRootPaths").
		Doc("Get a CDI API root paths").
		Returns(http.StatusOK, "OK", metav1.RootPaths{}).
		Returns(http.StatusNotFound, "Not Found", nil))

	// K8s needs the ability to query info about a specific API group
	ws.Route(ws.GET(groupPath).
		Produces("application/json").Writes(metav1.APIGroup{}).
		To(func(request *restful.Request, response *restful.Response) {
			response.WriteAsJson(uploadTokenAPIGroup())
		}).
		Operation("getAPIGroup").
		Doc("Get a CDI API Group").
		Returns(http.StatusOK, "OK", metav1.APIGroup{}).
		Returns(http.StatusNotFound, "Not Found", nil))

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
		Operation("getAPIGroup").
		Doc("Get a CDI API GroupList").
		Returns(http.StatusOK, "OK", metav1.APIGroupList{}).
		Returns(http.StatusNotFound, "Not Found", nil))

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
