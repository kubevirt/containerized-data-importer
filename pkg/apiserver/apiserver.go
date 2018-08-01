package apiserver

import (
	"crypto/rsa"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/emicklei/go-restful"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/cert/triple"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
	aggregatorclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	cdiv1 "kubevirt.io/containerized-data-importer/pkg/apis/cdicontroller/v1alpha1"
	. "kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	// selfsigned cert secret name
	apiCertSecretName = "cdi-api-certs"

	apiMutationWebhook = "cdi-api-mutator"
	tokenMutationPath  = "/uploadtoken-mutate"

	uploadTokenGroup   = "upload.cdi.kubevirt.io"
	uploadTokenVersion = "v1alpha1"

	apiServiceName = "cdi-api"

	certBytesValue        = "cert-bytes"
	keyBytesValue         = "key-bytes"
	signingCertBytesValue = "signing-cert-bytes"
)

type UploadApiServer interface {
	Start() error
}

type uploadApiApp struct {
	bindAddress string
	bindPort    uint

	client           *kubernetes.Clientset
	aggregatorClient *aggregatorclient.Clientset

	authorizor     CdiApiAuthorizor
	certsDirectory string

	signingCertBytes           []byte
	certBytes                  []byte
	keyBytes                   []byte
	clientCABytes              []byte
	requestHeaderClientCABytes []byte

	privateSigningKey   *rsa.PrivateKey
	publicEncryptionKey *rsa.PublicKey
}

func NewUploadApiServer(bindAddress string, bindPort uint, client *kubernetes.Clientset, aggregatorClient *aggregatorclient.Clientset, authorizor CdiApiAuthorizor) (UploadApiServer, error) {
	var err error
	app := &uploadApiApp{
		bindAddress:      bindAddress,
		bindPort:         bindPort,
		client:           client,
		aggregatorClient: aggregatorClient,
		authorizor:       authorizor,
	}
	app.certsDirectory, err = ioutil.TempDir("", "certsdir")
	if err != nil {
		glog.Fatalf("Unable to create certs temporary directory: %v\n", errors.WithStack(err))
	}

	err = app.getClientCert()
	if err != nil {
		return nil, errors.Errorf("Unable to get client cert: %v\n", errors.WithStack(err))
	}

	err = app.getSelfSignedCert()
	if err != nil {
		return nil, errors.Errorf("Unable to get self signed cert: %v\n", errors.WithStack(err))
	}

	err = app.createApiService()
	if err != nil {
		return nil, errors.Errorf("Unable to register aggregated api service: %v\n", errors.WithStack(err))
	}

	app.composeUploadTokenApi()

	restful.Filter(func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		var username = "-"
		if req.Request.URL.User != nil {
			if name := req.Request.URL.User.Username(); name != "" {
				username = name
			}
		}
		chain.ProcessFilter(req, resp)
		glog.V(Vuser).Infof("----------------------------")
		glog.V(Vuser).Infof("remoteAddress:%s", strings.Split(req.Request.RemoteAddr, ":")[0])
		glog.V(Vuser).Infof("username: %s", username)
		glog.V(Vuser).Infof("method: %s", req.Request.Method)
		glog.V(Vuser).Infof("url: %s", req.Request.URL.RequestURI())
		glog.V(Vuser).Infof("proto: %s", req.Request.Proto)
		glog.V(Vuser).Infof("headers: %v", req.Request.Header)
		glog.V(Vuser).Infof("statusCode: %d", resp.StatusCode())
		glog.V(Vuser).Infof("contentLength: %d", resp.ContentLength())

	})

	//err = app.createWebhook()
	//if err != nil {
	//	return nil, errors.Errorf("Unable to create webhook: %v\n", errors.WithStack(err))
	//}

	return app, nil
}

func (app *uploadApiApp) Start() error {
	return app.startTLS()
}

func deserializeStrings(in string) ([]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	var ret []string
	if err := json.Unmarshal([]byte(in), &ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func (app *uploadApiApp) getClientCert() error {
	authConfigMap, err := app.client.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get("extension-apiserver-authentication", metav1.GetOptions{})
	if err != nil {
		return err
	}

	clientCA, ok := authConfigMap.Data["client-ca-file"]
	if !ok {
		return errors.Errorf("client-ca-file value not found in auth config map.")
	}
	app.clientCABytes = []byte(clientCA)

	// request-header-ca-file doesn't always exist in all deployments.
	// set it if the value is set though.
	requestHeaderClientCA, ok := authConfigMap.Data["requestheader-client-ca-file"]
	if ok {
		app.requestHeaderClientCABytes = []byte(requestHeaderClientCA)
	}

	// This config map also contains information about what
	// headers our authorizor should inspect
	headers, ok := authConfigMap.Data["requestheader-username-headers"]
	if ok {
		headerList, err := deserializeStrings(headers)
		if err != nil {
			return err
		}
		app.authorizor.AddUserHeaders(headerList)
	}

	headers, ok = authConfigMap.Data["requestheader-group-headers"]
	if ok {
		headerList, err := deserializeStrings(headers)
		if err != nil {
			return err
		}
		app.authorizor.AddGroupHeaders(headerList)
	}

	headers, ok = authConfigMap.Data["requestheader-extra-headers-prefix"]
	if ok {
		headerList, err := deserializeStrings(headers)
		if err != nil {
			return err
		}
		app.authorizor.AddExtraPrefixHeaders(headerList)
	}

	return nil
}

func (app *uploadApiApp) getSelfSignedCert() error {
	var ok bool

	namespace := GetNamespace()
	generateCerts := false
	secret, err := app.client.CoreV1().Secrets(namespace).Get(apiCertSecretName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			generateCerts = true
		} else {
			return err
		}
	}

	if generateCerts {
		// Generate new certs if secret doesn't already exist
		caKeyPair, _ := triple.NewCA("kubecdi.io")
		keyPair, _ := triple.NewServerKeyPair(
			caKeyPair,
			"cdi-api."+namespace+".pod.cluster.local",
			"cdi-api",
			namespace,
			"cluster.local",
			nil,
			nil,
		)

		app.keyBytes = cert.EncodePrivateKeyPEM(keyPair.Key)
		app.certBytes = cert.EncodeCertPEM(keyPair.Cert)
		app.signingCertBytes = cert.EncodeCertPEM(caKeyPair.Cert)

		secret := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      apiCertSecretName,
				Namespace: namespace,
				Labels: map[string]string{
					CDI_COMPONENT_LABEL: "cdi-api-aggregator",
				},
			},
			Type: "Opaque",
			Data: map[string][]byte{
				certBytesValue:        app.certBytes,
				keyBytesValue:         app.keyBytes,
				signingCertBytesValue: app.signingCertBytes,
			},
		}
		_, err := app.client.CoreV1().Secrets(namespace).Create(&secret)
		if err != nil {
			return err
		}
	} else {
		// retrieve self signed cert info from secret

		app.certBytes, ok = secret.Data[certBytesValue]
		if !ok {
			return errors.Errorf("%s value not found in %s cdi-api secret", certBytesValue, apiCertSecretName)
		}
		app.keyBytes, ok = secret.Data[keyBytesValue]
		if !ok {
			return errors.Errorf("%s value not found in %s cdi-api secret", keyBytesValue, apiCertSecretName)
		}
		app.signingCertBytes, ok = secret.Data[signingCertBytesValue]
		if !ok {
			return errors.Errorf("%s value not found in %s cdi-api secret", signingCertBytesValue, apiCertSecretName)
		}
	}

	obj, err := cert.ParsePrivateKeyPEM(app.keyBytes)
	privateKey, ok := obj.(*rsa.PrivateKey)
	if err != nil {
		return err
	}
	if !ok {
		return errors.Errorf("unable to parse private key")
	}

	err = RecordApiPublicKey(app.client, &privateKey.PublicKey)
	if err != nil {
		return err
	}

	app.privateSigningKey = privateKey

	publicKey, exists, err := GetUploadProxyPublicKey(app.client)
	if err != nil {
		return err
	} else if !exists {
		return errors.Errorf("upload proxy has not generated encryption key yet")
	}

	app.publicEncryptionKey = publicKey

	return nil
}

func (app *uploadApiApp) startTLS() error {

	errors := make(chan error)

	keyFile := filepath.Join(app.certsDirectory, "/key.pem")
	certFile := filepath.Join(app.certsDirectory, "/cert.pem")
	signingCertFile := filepath.Join(app.certsDirectory, "/signingCert.pem")
	clientCAFile := filepath.Join(app.certsDirectory, "/clientCA.crt")

	// Write the certs to disk
	err := ioutil.WriteFile(clientCAFile, app.clientCABytes, 0600)
	if err != nil {
		return err
	}

	if len(app.requestHeaderClientCABytes) != 0 {
		f, err := os.OpenFile(clientCAFile, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = f.Write(app.requestHeaderClientCABytes)
		if err != nil {
			return err
		}
	}

	err = ioutil.WriteFile(keyFile, app.keyBytes, 0600)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(certFile, app.certBytes, 0600)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(signingCertFile, app.signingCertBytes, 0600)
	if err != nil {
		return err
	}

	// create the client CA pool.
	// This ensures we're talking to the k8s api server
	pool, err := cert.NewPool(clientCAFile)
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{
		ClientCAs:  pool,
		ClientAuth: tls.RequestClientCert,
	}
	tlsConfig.BuildNameToCertificate()

	go func() {
		server := &http.Server{
			Addr:      fmt.Sprintf("%s:%d", app.bindAddress, app.bindPort),
			TLSConfig: tlsConfig,
		}

		errors <- server.ListenAndServeTLS(certFile, keyFile)
	}()

	// wait for server to exit
	return <-errors
}

func toAdmissionResponse(err error) *v1beta1.AdmissionResponse {
	return &v1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}

type admitFunc func(*v1beta1.AdmissionReview, *rsa.PrivateKey, *rsa.PublicKey) *v1beta1.AdmissionResponse

func mutateUploadTokens(ar *v1beta1.AdmissionReview, signingKey *rsa.PrivateKey, encryptionKey *rsa.PublicKey) *v1beta1.AdmissionResponse {
	glog.V(Vadmin).Info("adding token to upload token crd")
	token := cdiv1.UploadToken{}

	raw := ar.Request.Object.Raw
	err := json.Unmarshal(raw, &token)
	if err != nil {
		glog.Error(err)
		return toAdmissionResponse(err)
	}

	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true

	if token.Spec.PvcName == "" {
		return toAdmissionResponse(errors.Errorf("no pvcName set on UploadToken spec"))
	}

	encryptedTokenData, err := GenerateToken(token.Spec.PvcName, token.Namespace, encryptionKey, signingKey)
	if err != nil {
		glog.Error(err)
		return toAdmissionResponse(err)
	}
	patch := fmt.Sprintf("[{ \"op\": \"add\", \"path\": \"/status\", \"value\": { \"token\" : \"%s\" } }]", encryptedTokenData)

	reviewResponse.Patch = []byte(patch)

	pt := v1beta1.PatchTypeJSONPatch
	reviewResponse.PatchType = &pt
	return &reviewResponse
}

func getAdmissionReview(r *http.Request) (*v1beta1.AdmissionReview, error) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		return nil, fmt.Errorf("contentType=%s, expect application/json", contentType)
	}

	ar := &v1beta1.AdmissionReview{}
	err := json.Unmarshal(body, ar)
	return ar, err
}

func (app *uploadApiApp) serve(resp http.ResponseWriter, req *http.Request, admit admitFunc) {
	response := v1beta1.AdmissionReview{}
	review, err := getAdmissionReview(req)

	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	reviewResponse := admit(review, app.privateSigningKey, app.publicEncryptionKey)
	if reviewResponse != nil {
		response.Response = reviewResponse
		response.Response.UID = review.Request.UID
	}
	// reset the Object and OldObject, they are not needed in a response.
	review.Request.Object = runtime.RawExtension{}
	review.Request.OldObject = runtime.RawExtension{}

	responseBytes, err := json.Marshal(response)
	if err != nil {
		glog.Error(err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, err := resp.Write(responseBytes); err != nil {
		glog.Error(err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	resp.WriteHeader(http.StatusOK)
}

func (app *uploadApiApp) serveMutateUploadTokens(w http.ResponseWriter, r *http.Request) {
	app.serve(w, r, mutateUploadTokens)
}

func (app *uploadApiApp) uploadHandler(request *restful.Request, response *restful.Response) {

	allowed, reason, err := app.authorizor.Authorize(request)
	if err != nil {
		glog.Error(err)
		response.WriteHeader(http.StatusInternalServerError)
		return
	} else if !allowed {
		glog.Infof("Rejected Request: %s", reason)
		response.WriteErrorString(http.StatusUnauthorized, reason)
		return
	}

	namespace := request.PathParameter("namespace")
	defer request.Request.Body.Close()
	body, err := ioutil.ReadAll(request.Request.Body)
	if err != nil {
		glog.Error(err)
		response.WriteError(http.StatusBadRequest, err)
	}

	uploadToken := &cdiv1.UploadToken{}
	err = json.Unmarshal(body, uploadToken)
	if err != nil {
		glog.Error(err)
		response.WriteError(http.StatusBadRequest, err)
	}

	encryptedTokenData, err := GenerateToken(uploadToken.Spec.PvcName, namespace, app.publicEncryptionKey, app.privateSigningKey)

	uploadToken.Status.Token = encryptedTokenData
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
	return apiGroup
}

func (app *uploadApiApp) composeUploadTokenApi() {
	objPointer := &cdiv1.UploadToken{}
	objExample := reflect.ValueOf(objPointer).Elem().Interface()
	objKind := "uploadtoken"

	groupPath := fmt.Sprintf("/apis/%s/%s", uploadTokenGroup, uploadTokenVersion)
	resourcePath := fmt.Sprintf("/apis/%s/%s", uploadTokenGroup, uploadTokenVersion)
	createPath := fmt.Sprintf("/namespaces/{namespace:[a-z0-9][a-z0-9\\-]*}/%s", objKind)

	uploadTokenWs := new(restful.WebService)
	uploadTokenWs.Doc("The CDI Upload API.")
	uploadTokenWs.Path(resourcePath)

	uploadTokenWs.Route(uploadTokenWs.POST(createPath).
		Produces("application/json").
		Consumes("application/json").
		Operation("createNamespaced"+objKind).
		To(app.uploadHandler).Reads(objExample).Writes(objExample).
		Doc("Create an UploadToken object.").
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
			list.GroupVersion = uploadTokenGroup + "/" + uploadTokenVersion
			list.APIVersion = uploadTokenVersion
			list.APIResources = append(list.APIResources, metav1.APIResource{
				Name:         "UploadToken",
				SingularName: "uploadtoken",
				Namespaced:   true,
				Group:        uploadTokenGroup,
				Version:      uploadTokenVersion,
				Kind:         "UploadToken",
				Verbs:        []string{"create"},
				ShortNames:   []string{"ut", "uts"},
			})
			response.WriteAsJson(list)
		}).
		Operation("getAPIResources").
		Doc("Get a CDI Upload API resources").
		Returns(http.StatusOK, "OK", metav1.APIResourceList{}).
		Returns(http.StatusNotFound, "Not Found", nil))

	restful.Add(uploadTokenWs)

	ws := new(restful.WebService)

	// K8s needs the ability to query info about a specific API group
	ws.Route(ws.GET(groupPath).
		Produces("application/json").Writes(metav1.APIGroup{}).
		To(func(request *restful.Request, response *restful.Response) {
			response.WriteAsJson(uploadTokenAPIGroup())
		}).
		Operation("getAPIGroup").
		Doc("Get a CDI Upload API Group").
		Returns(http.StatusOK, "OK", metav1.APIGroup{}).
		Returns(http.StatusNotFound, "Not Found", nil))

	// K8s needs the ability to query the list of API groups this endpoint supports
	ws.Route(ws.GET("apis").
		Produces("application/json").Writes(metav1.APIGroupList{}).
		To(func(request *restful.Request, response *restful.Response) {
			list := &metav1.APIGroupList{}
			list.Kind = "APIGroupList"
			list.Groups = append(list.Groups, uploadTokenAPIGroup())
			response.WriteAsJson(list)
		}).
		Operation("getAPIGroup").
		Doc("Get a CDI Upload API GroupList").
		Returns(http.StatusOK, "OK", metav1.APIGroupList{}).
		Returns(http.StatusNotFound, "Not Found", nil))

	restful.Add(ws)
}

func (app *uploadApiApp) createApiService() error {
	namespace := GetNamespace()
	apiName := uploadTokenVersion + "." + uploadTokenGroup

	registerApiService := false

	apiService, err := app.aggregatorClient.ApiregistrationV1beta1().APIServices().Get(apiName, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			registerApiService = true
		} else {
			return err
		}
	}

	newApiService := &apiregistrationv1beta1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      apiName,
			Namespace: namespace,
			Labels: map[string]string{
				CDI_COMPONENT_LABEL: apiServiceName,
			},
		},
		Spec: apiregistrationv1beta1.APIServiceSpec{
			Service: &apiregistrationv1beta1.ServiceReference{
				Namespace: namespace,
				Name:      apiServiceName,
			},
			Group:                uploadTokenGroup,
			Version:              uploadTokenVersion,
			CABundle:             app.signingCertBytes,
			GroupPriorityMinimum: 1000,
			VersionPriority:      15,
		},
	}

	if registerApiService {
		_, err = app.aggregatorClient.ApiregistrationV1beta1().APIServices().Create(newApiService)
		if err != nil {
			return err
		}
	} else {
		if apiService.Spec.Service != nil && apiService.Spec.Service.Namespace != namespace {
			return fmt.Errorf("apiservice [%s] is already registered in a different namespace. Existing apiservice registration must be deleted before virt-api can proceed.", apiName)
		}

		// Always update spec to latest.
		apiService.Spec = newApiService.Spec
		_, err := app.aggregatorClient.ApiregistrationV1beta1().APIServices().Update(apiService)
		if err != nil {
			return err
		}
	}
	return nil
}

func (app *uploadApiApp) createWebhook() error {
	namespace := GetNamespace()
	registerWebhook := false

	tokenPath := tokenMutationPath

	webhookRegistration, err := app.client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Get(apiMutationWebhook, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			registerWebhook = true
		} else {
			return err
		}
	}

	webHooks := []admissionregistrationv1beta1.Webhook{
		{
			Name: "uploadtoken-mutator.cdi.kubevirt.io",
			Rules: []admissionregistrationv1beta1.RuleWithOperations{{
				Operations: []admissionregistrationv1beta1.OperationType{admissionregistrationv1beta1.Create},
				Rule: admissionregistrationv1beta1.Rule{
					APIGroups:   []string{cdiv1.SchemeGroupVersion.Group},
					APIVersions: []string{cdiv1.SchemeGroupVersion.Version},
					Resources:   []string{"uploadtokens"},
				},
			}},
			ClientConfig: admissionregistrationv1beta1.WebhookClientConfig{
				Service: &admissionregistrationv1beta1.ServiceReference{
					Namespace: namespace,
					Name:      apiServiceName,
					Path:      &tokenPath,
				},
				CABundle: app.signingCertBytes,
			},
		},
	}

	if registerWebhook {
		_, err := app.client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Create(&admissionregistrationv1beta1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: apiMutationWebhook,
				Labels: map[string]string{
					CDI_COMPONENT_LABEL: apiMutationWebhook,
				},
			},
			Webhooks: webHooks,
		})
		if err != nil {
			return err
		}
	} else {
		for _, webhook := range webhookRegistration.Webhooks {
			if webhook.ClientConfig.Service != nil && webhook.ClientConfig.Service.Namespace != namespace {
				return fmt.Errorf("Webhook [%s] is already registered using services endpoints in a different namespace. Existing webhook registration must be deleted before virt-api can proceed.", apiMutationWebhook)
			}
		}

		// update registered webhook with our data
		webhookRegistration.Webhooks = webHooks

		_, err := app.client.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().Update(webhookRegistration)
		if err != nil {
			return err
		}
	}

	http.HandleFunc(tokenPath, func(w http.ResponseWriter, r *http.Request) {
		app.serveMutateUploadTokens(w, r)
	})

	return nil
}
