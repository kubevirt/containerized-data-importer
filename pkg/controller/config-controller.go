package controller

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	ocpconfigv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	cc "kubevirt.io/containerized-data-importer/pkg/controller/common"
	"kubevirt.io/containerized-data-importer/pkg/operator"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
)

// AnnConfigAuthority is the annotation specifying a resource as the CDIConfig authority
const (
	AnnConfigAuthority = "cdi.kubevirt.io/configAuthority"

	errResourceDoesntExist     = "ErrResourceDoesntExist"
	messageResourceDoesntExist = "Resource managed by %q doesn't exist"

	defaultCPULimit   = "750m"
	defaultMemLimit   = "600M"
	defaultCPURequest = "100m"
	defaultMemRequest = "60M"

	rootCertificateConfigMap = "kube-root-ca.crt"
)

// CDIConfigReconciler members
type CDIConfigReconciler struct {
	client client.Client
	// use this for getting any resources not in the install namespace or cluster scope
	uncachedClient         client.Client
	recorder               record.EventRecorder
	scheme                 *runtime.Scheme
	log                    logr.Logger
	uploadProxyServiceName string
	configName             string
	cdiNamespace           string
	installerLabels        map[string]string
}

// Reconcile the reconcile loop for the CDIConfig object.
func (r *CDIConfigReconciler) Reconcile(_ context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.log.WithValues("CDIConfig", req.NamespacedName)
	log.Info("reconciling CDIConfig")

	config, err := r.createCDIConfig()
	if err != nil {
		log.Error(err, "Unable to create CDIConfig")
		return reconcile.Result{}, err
	}
	// Keep a copy of the original for comparison later.
	currentConfigCopy := config.DeepCopyObject()

	config.Status.Preallocation = config.Spec.Preallocation != nil && *config.Spec.Preallocation

	// ignore whatever is in config spec and set to operator view
	if err := r.setOperatorParams(config); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileUploadProxy(config); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileStorageClass(config); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileDefaultPodResourceRequirements(config); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileImagePullSecrets(config); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileFilesystemOverhead(config); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileImportProxy(config); err != nil {
		return reconcile.Result{}, err
	}

	if !reflect.DeepEqual(currentConfigCopy, config) {
		// Updates have happened, update CDIConfig.
		log.Info("Updating CDIConfig", "CDIConfig.Name", config.Name, "config", config)
		if err := r.client.Update(context.TODO(), config); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *CDIConfigReconciler) setOperatorParams(config *cdiv1.CDIConfig) error {
	util.SetRecommendedLabels(config, r.installerLabels, "cdi-controller")

	cdiCR, err := cc.GetActiveCDI(context.TODO(), r.client)
	if err != nil {
		return err
	}

	if cdiCR == nil {
		return nil
	}

	if _, ok := cdiCR.Annotations[AnnConfigAuthority]; !ok {
		return nil
	}

	if cdiCR.Spec.Config == nil {
		config.Spec = cdiv1.CDIConfigSpec{}
	} else {
		config.Spec = *cdiCR.Spec.Config
	}

	return nil
}

func (r *CDIConfigReconciler) reconcileUploadProxy(config *cdiv1.CDIConfig) error {
	log := r.log.WithName("CDIconfig").WithName("UploadProxyReconcile")
	config.Status.UploadProxyURL = config.Spec.UploadProxyURLOverride
	// No override, try Ingress
	if config.Status.UploadProxyURL == nil {
		ingress, err := r.reconcileIngress(config)
		if err != nil {
			log.Error(err, "Unable to reconcile Ingress")
			return err
		}

		if ingress != nil {
			if err := r.reconcileUploadProxyIngressCA(config, *ingress); err != nil {
				log.Error(err, "Unable to reconcile Ingress CA")
				return fmt.Errorf("unable to reconcile Ingress CA: %w", err)
			}
		}
	}
	// No override or Ingress, try Route
	if config.Status.UploadProxyURL == nil {
		if err := r.reconcileRoute(config); err != nil {
			log.Error(err, "Unable to reconcile Routes")
			return err
		}

		if err := r.reconcileUploadProxyRouteCA(config); err != nil {
			log.Error(err, "Unable to reconcile Route CA")
			return fmt.Errorf("unable to reconcile Route CA: %w", err)
		}
	}
	return nil
}

func (r *CDIConfigReconciler) reconcileUploadProxyIngressCA(config *cdiv1.CDIConfig, ingress networkingv1.Ingress) error {
	log := r.log.WithName("CDIconfig").WithName("UploadProxyIngressCAReconcile")

	url := config.Status.UploadProxyURL
	if url == nil || *url == "" {
		return nil
	}

	var secretName string
	i := slices.IndexFunc(ingress.Spec.TLS, func(tls networkingv1.IngressTLS) bool { return tls.SecretName != "" })
	if i == -1 {
		log.Info("Secret name not found in Ingress")
		config.Status.UploadProxyCA = nil
		return nil
	}
	secretName = ingress.Spec.TLS[i].SecretName

	var secret corev1.Secret
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: r.cdiNamespace}, &secret)
	if err != nil {
		return fmt.Errorf("unable to get secret %q: %v", secretName, err)
	}

	certBytes, ok := secret.Data["tls.crt"]
	if !ok {
		log.Info(fmt.Sprintf("Secret %q does not contain %q", secretName, "tls.crt"))
		config.Status.UploadProxyCA = nil
		return nil
	}

	certs, err := cert.ParseCertsPEM(certBytes)
	if err != nil {
		return fmt.Errorf("unable to parse tls.crt: %v", err)
	}

	s, err := findCertByHostName(*config.Status.UploadProxyURL, certs)
	if err != nil {
		return err
	} else if s == "" {
		log.Info("No matching valid certificate found for upload proxy URL", "UploadProxyURL", *config.Status.UploadProxyURL)
		config.Status.UploadProxyCA = nil
		return nil
	}

	log.Info("Setting upload proxy CA", "UploadProxyCA", s)
	config.Status.UploadProxyCA = &s
	return nil
}

func (r *CDIConfigReconciler) reconcileUploadProxyRouteCA(config *cdiv1.CDIConfig) error {
	log := r.log.WithName("CDIconfig").WithName("UploadProxyRouteCAReconcile")

	if config.Status.UploadProxyURL == nil || *config.Status.UploadProxyURL == "" {
		log.Info("No upload proxy URL found, setting upload proxy CA to blank")
		config.Status.UploadProxyCA = nil
		return nil
	}

	var cm corev1.ConfigMap
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: rootCertificateConfigMap, Namespace: r.cdiNamespace}, &cm)
	if err != nil {
		log.Info(fmt.Sprintf("Could not get certificates: %v", err))
		config.Status.UploadProxyCA = nil
		return nil
	}

	rawCert, ok := cm.Data["ca.crt"]
	if !ok {
		log.Info(fmt.Sprintf("Config map %q does not contain %q", rootCertificateConfigMap, "ca.crt"))
		config.Status.UploadProxyCA = nil
		return nil
	}

	certs, err := cert.ParseCertsPEM([]byte(rawCert))
	if err != nil {
		return fmt.Errorf("unable to parse ca.crt: %v", err)
	}

	s, err := findCertByHostName(*config.Status.UploadProxyURL, certs)
	if err != nil {
		return err
	} else if s == "" {
		log.Info("No matching valid certificate found for upload proxy URL", "UploadProxyURL", *config.Status.UploadProxyURL)
		config.Status.UploadProxyCA = nil
		return nil
	}

	log.Info("Setting upload proxy CA", "UploadProxyCA", s)
	config.Status.UploadProxyCA = &s
	return nil
}

func findCertByHostName(hostName string, certs []*x509.Certificate) (string, error) {
	now := time.Now()
	var latestValidCert *x509.Certificate
	for _, cert := range certs {
		// Check validity
		if now.After(cert.NotAfter) {
			continue
		}
		if now.Before(cert.NotBefore) {
			continue
		}
		if err := cert.VerifyHostname(hostName); err != nil {
			continue
		}

		// Check if this is the cert with the latest expiration date
		if latestValidCert == nil {
			latestValidCert = cert
			continue
		}
		if latestValidCert.NotAfter.After(cert.NotAfter) {
			continue
		}
		latestValidCert = cert
	}

	if latestValidCert != nil {
		return buildPemFromCert(latestValidCert, certs)
	}

	if len(certs) > 0 {
		return buildPemFromAllCerts(certs)
	}

	return "", nil
}

func buildPemFromCert(matchingCert *x509.Certificate, allCerts []*x509.Certificate) (string, error) {
	pemOut := strings.Builder{}

	if err := pem.Encode(&pemOut, &pem.Block{Type: "CERTIFICATE", Bytes: matchingCert.Raw}); err != nil {
		return "", fmt.Errorf("could not encode certificate: %w", err)
	}

	if matchingCert.Issuer.CommonName != matchingCert.Subject.CommonName && !matchingCert.IsCA {
		//lookup issuer recursively, if not found a blank is returned.
		chain, err := findCertByHostName(matchingCert.Issuer.CommonName, allCerts)
		if err != nil {
			return "", err
		}

		if _, err := pemOut.WriteString(chain); err != nil {
			return "", fmt.Errorf("could not write issuer certificate: %w", err)
		}
	}

	return strings.TrimSpace(pemOut.String()), nil
}

func buildPemFromAllCerts(allCerts []*x509.Certificate) (string, error) {
	now := time.Now()
	pemOut := strings.Builder{}
	for _, cert := range allCerts {
		if now.After(cert.NotAfter) {
			continue
		}

		if now.Before(cert.NotBefore) {
			continue
		}

		if err := pem.Encode(&pemOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
			return "", fmt.Errorf("could not encode certificate: %w", err)
		}
	}

	return strings.TrimSpace(pemOut.String()), nil
}

func (r *CDIConfigReconciler) reconcileIngress(config *cdiv1.CDIConfig) (*networkingv1.Ingress, error) {
	log := r.log.WithName("CDIconfig").WithName("IngressReconcile")
	ingressList := &networkingv1.IngressList{}
	if err := r.client.List(context.TODO(), ingressList, &client.ListOptions{Namespace: r.cdiNamespace}); cc.IgnoreIsNoMatchError(err) != nil {
		return nil, err
	}
	for _, ingress := range ingressList.Items {
		ingressURL := getURLFromIngress(&ingress, r.uploadProxyServiceName)
		if ingressURL != "" {
			log.Info("Setting upload proxy url", "IngressURL", ingressURL)
			config.Status.UploadProxyURL = &ingressURL
			return &ingress, nil
		}
	}
	log.Info("No ingress found, setting to blank", "IngressURL", "")
	config.Status.UploadProxyURL = nil
	return nil, nil
}

func (r *CDIConfigReconciler) reconcileRoute(config *cdiv1.CDIConfig) error {
	log := r.log.WithName("CDIconfig").WithName("RouteReconcile")
	routeList := &routev1.RouteList{}
	if err := r.client.List(context.TODO(), routeList, &client.ListOptions{Namespace: r.cdiNamespace}); cc.IgnoreIsNoMatchError(err) != nil {
		return err
	}
	for _, route := range routeList.Items {
		routeURL := getURLFromRoute(&route, r.uploadProxyServiceName)
		if routeURL != "" {
			log.Info("Setting upload proxy url", "RouteURL", routeURL)
			config.Status.UploadProxyURL = &routeURL
			return nil
		}
	}
	log.Info("No route found, setting to blank", "RouteURL", "")
	config.Status.UploadProxyURL = nil
	return nil
}

func (r *CDIConfigReconciler) reconcileStorageClass(config *cdiv1.CDIConfig) error {
	log := r.log.WithName("CDIconfig").WithName("StorageClassReconcile")
	storageClassList := &storagev1.StorageClassList{}
	if err := r.client.List(context.TODO(), storageClassList, &client.ListOptions{}); err != nil {
		return err
	}

	// Check config for scratch space class
	if config.Spec.ScratchSpaceStorageClass != nil {
		for _, storageClass := range storageClassList.Items {
			if storageClass.Name == *config.Spec.ScratchSpaceStorageClass {
				log.Info("Setting scratch space to override", "storageClass.Name", storageClass.Name)
				config.Status.ScratchSpaceStorageClass = storageClass.Name
				return nil
			}
		}
	}
	// Check for default storage class.
	for _, storageClass := range storageClassList.Items {
		if defaultClassValue, ok := storageClass.Annotations[cc.AnnDefaultStorageClass]; ok {
			if defaultClassValue == "true" {
				log.Info("Setting scratch space to default", "storageClass.Name", storageClass.Name)
				config.Status.ScratchSpaceStorageClass = storageClass.Name
				return nil
			}
		}
	}
	log.Info("No default storage class found, setting scratch space to blank")
	// No storage class found, blank it out.
	config.Status.ScratchSpaceStorageClass = ""
	return nil
}

func (r *CDIConfigReconciler) reconcileImagePullSecrets(config *cdiv1.CDIConfig) error {
	config.Status.ImagePullSecrets = config.Spec.ImagePullSecrets
	return nil
}

func (r *CDIConfigReconciler) reconcileDefaultPodResourceRequirements(config *cdiv1.CDIConfig) error {
	cpuLimit, _ := resource.ParseQuantity(defaultCPULimit)
	memLimit, _ := resource.ParseQuantity(defaultMemLimit)
	cpuRequest, _ := resource.ParseQuantity(defaultCPURequest)
	memRequest, _ := resource.ParseQuantity(defaultMemRequest)
	config.Status.DefaultPodResourceRequirements = &v1.ResourceRequirements{
		Limits: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU:    cpuLimit,
			v1.ResourceMemory: memLimit,
		},
		Requests: map[v1.ResourceName]resource.Quantity{
			v1.ResourceCPU:    cpuRequest,
			v1.ResourceMemory: memRequest,
		},
	}

	if config.Spec.PodResourceRequirements != nil {
		if config.Spec.PodResourceRequirements.Limits != nil {
			if cpu, exist := config.Spec.PodResourceRequirements.Limits[v1.ResourceCPU]; exist {
				config.Status.DefaultPodResourceRequirements.Limits[v1.ResourceCPU] = cpu
			}

			if memory, exist := config.Spec.PodResourceRequirements.Limits[v1.ResourceMemory]; exist {
				config.Status.DefaultPodResourceRequirements.Limits[v1.ResourceMemory] = memory
			}
		}

		if config.Spec.PodResourceRequirements.Requests != nil {
			if cpu, exist := config.Spec.PodResourceRequirements.Requests[v1.ResourceCPU]; exist {
				config.Status.DefaultPodResourceRequirements.Requests[v1.ResourceCPU] = cpu
			}

			if memory, exist := config.Spec.PodResourceRequirements.Requests[v1.ResourceMemory]; exist {
				config.Status.DefaultPodResourceRequirements.Requests[v1.ResourceMemory] = memory
			}
		}
	}

	return nil
}

func (r *CDIConfigReconciler) reconcileFilesystemOverhead(config *cdiv1.CDIConfig) error {
	var globalOverhead cdiv1.Percent = common.DefaultGlobalOverhead
	var perStorageConfig = make(map[string]cdiv1.Percent)

	log := r.log.WithName("CDIconfig").WithName("FilesystemOverhead")

	// Avoid nil maps and segfaults for the initial case, where filesystemOverhead
	// is nil for both the spec and the status.
	if config.Status.FilesystemOverhead == nil {
		log.Info("No filesystem overhead found in status, initializing to defaults")
		config.Status.FilesystemOverhead = &cdiv1.FilesystemOverhead{
			Global:       globalOverhead,
			StorageClass: make(map[string]cdiv1.Percent),
		}
	}

	if config.Spec.FilesystemOverhead != nil {
		if valid, _ := validOverhead(config.Spec.FilesystemOverhead.Global); valid {
			globalOverhead = config.Spec.FilesystemOverhead.Global
		}
		if config.Spec.FilesystemOverhead.StorageClass != nil {
			perStorageConfig = config.Spec.FilesystemOverhead.StorageClass
		}
	}

	// Set status global overhead
	config.Status.FilesystemOverhead.Global = globalOverhead

	// Set status per-storageClass overhead
	storageClassList := &storagev1.StorageClassList{}
	if err := r.client.List(context.TODO(), storageClassList, &client.ListOptions{}); err != nil {
		return err
	}
	config.Status.FilesystemOverhead.StorageClass = make(map[string]cdiv1.Percent)
	for _, storageClass := range storageClassList.Items {
		storageClassName := storageClass.GetName()
		storageClassNameOverhead, found := perStorageConfig[storageClassName]

		if found {
			valid, err := validOverhead(storageClassNameOverhead)
			if !valid {
				return err
			}
			config.Status.FilesystemOverhead.StorageClass[storageClassName] = storageClassNameOverhead
		} else {
			config.Status.FilesystemOverhead.StorageClass[storageClassName] = globalOverhead
		}
	}

	return nil
}

func validOverhead(overhead cdiv1.Percent) (bool, error) {
	return regexp.MatchString(`^(0(?:\.\d{1,3})?|1)$`, string(overhead))
}

// createCDIConfig creates a new instance of the CDIConfig object if it doesn't exist already, and returns the existing one if found.
// It also sets the operator to be the owner of the CDIConfig object.
func (r *CDIConfigReconciler) createCDIConfig() (*cdiv1.CDIConfig, error) {
	config := &cdiv1.CDIConfig{}
	if err := r.uncachedClient.Get(context.TODO(), types.NamespacedName{Name: r.configName}, config); err != nil {
		if k8serrors.IsNotFound(err) {
			config = cc.MakeEmptyCDIConfigSpec(r.configName)
			if err := operator.SetOwnerRuntime(r.uncachedClient, config); err != nil {
				return nil, err
			}
			util.SetRecommendedLabels(config, r.installerLabels, "cdi-controller")
			if err := r.client.Create(context.TODO(), config); err != nil {
				if k8serrors.IsAlreadyExists(err) {
					config := &cdiv1.CDIConfig{}
					if err := r.uncachedClient.Get(context.TODO(), types.NamespacedName{Name: r.configName}, config); err == nil {
						return config, nil
					}
					return nil, err
				}
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return config, nil
}

func (r *CDIConfigReconciler) reconcileImportProxy(config *cdiv1.CDIConfig) error {
	config.Status.ImportProxy = config.Spec.ImportProxy

	// Avoid nil pointers and segfaults for the initial case, where ImportProxy is nil for both the spec and the status.
	if config.Status.ImportProxy == nil {
		config.Status.ImportProxy = &cdiv1.ImportProxy{
			HTTPProxy:      new(string),
			HTTPSProxy:     new(string),
			NoProxy:        new(string),
			TrustedCAProxy: new(string),
		}

		// Try Openshift cluster wide proxy only if the CDIConfig default config is empty
		clusterWideProxy, err := getClusterWideProxy(r.client)
		if err != nil {
			return err
		}
		config.Status.ImportProxy.HTTPProxy = &clusterWideProxy.Status.HTTPProxy
		config.Status.ImportProxy.HTTPSProxy = &clusterWideProxy.Status.HTTPSProxy
		config.Status.ImportProxy.NoProxy = &clusterWideProxy.Status.NoProxy
		if err := r.reconcileImportProxyCAConfigMap(config, clusterWideProxy); err != nil {
			return err
		}
		config.Status.ImportProxy.TrustedCAProxy = &clusterWideProxy.Spec.TrustedCA.Name
	}
	return nil
}

// Create/Update a configmap with the CA certificates in the controllor context with the cluster-wide proxy CA certificates to be used by the importer pod
func (r *CDIConfigReconciler) reconcileImportProxyCAConfigMap(config *cdiv1.CDIConfig, clusterWideProxy *ocpconfigv1.Proxy) error {
	cmOldName := config.Status.ImportProxy.TrustedCAProxy
	cmName := clusterWideProxy.Spec.TrustedCA.Name
	client := r.uncachedClient

	// Delete old ConfigMap if name changed
	if cmOldName != nil && *cmOldName != "" && *cmOldName != cmName {
		if err := client.Delete(context.TODO(), r.createProxyConfigMap(*cmOldName, "")); err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
	}
	if cmName == "" {
		return nil
	}

	clusterWideProxyConfigMap := &v1.ConfigMap{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: cmName, Namespace: ClusterWideProxyConfigMapNameSpace}, clusterWideProxyConfigMap); err != nil {
		if k8serrors.IsNotFound(err) {
			msg := fmt.Sprintf(messageResourceDoesntExist, cmName)
			r.recorder.Event(clusterWideProxy, v1.EventTypeWarning, errResourceDoesntExist, msg)
		}
		return err
	}
	// Copy the cluster-wide proxy CA certificates to the importer pod proxy CA certificates configMap
	certBytes, ok := clusterWideProxyConfigMap.Data[ClusterWideProxyConfigMapKey]
	if !ok {
		return fmt.Errorf("no cluster-wide proxy CA certificate")
	}
	configMap := &v1.ConfigMap{}
	if err := client.Get(context.TODO(), types.NamespacedName{Name: cmName, Namespace: r.cdiNamespace}, configMap); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		proxyConfigMap := r.createProxyConfigMap(cmName, certBytes)
		util.SetRecommendedLabels(proxyConfigMap, r.installerLabels, "cdi-controller")
		if err := client.Create(context.TODO(), proxyConfigMap); err != nil {
			return err
		}
		return nil
	}
	configMap.Data[common.ImportProxyConfigMapKey] = certBytes
	util.SetRecommendedLabels(configMap, r.installerLabels, "cdi-controller")
	if err := client.Update(context.TODO(), configMap); err != nil {
		return err
	}
	return nil
}

func (r *CDIConfigReconciler) createProxyConfigMap(cmName, cert string) *v1.ConfigMap {
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: r.cdiNamespace},
		Data: map[string]string{common.ImportProxyConfigMapKey: cert},
	}
}

// Init initializes a CDIConfig object.
func (r *CDIConfigReconciler) Init() error {
	_, err := r.createCDIConfig()
	return err
}

// NewConfigController creates a new instance of the config controller.
func NewConfigController(mgr manager.Manager, log logr.Logger, uploadProxyServiceName, configName string, installerLabels map[string]string) (controller.Controller, error) {
	uncachedClient, err := client.New(mgr.GetConfig(), client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	if err != nil {
		return nil, err
	}
	reconciler := &CDIConfigReconciler{
		client:                 mgr.GetClient(),
		uncachedClient:         uncachedClient,
		recorder:               mgr.GetEventRecorderFor("config-controller"),
		scheme:                 mgr.GetScheme(),
		log:                    log.WithName("config-controller"),
		uploadProxyServiceName: uploadProxyServiceName,
		configName:             configName,
		cdiNamespace:           util.GetNamespace(),
		installerLabels:        installerLabels,
	}

	configController, err := controller.New("config-controller", mgr, controller.Options{
		MaxConcurrentReconciles: 3,
		Reconciler:              reconciler,
	})
	if err != nil {
		return nil, err
	}
	if err := addConfigControllerWatches(mgr, configController, reconciler.cdiNamespace, configName, uploadProxyServiceName, log); err != nil {
		return nil, err
	}
	if err := reconciler.Init(); err != nil {
		log.Error(err, "Unable to initialize CDIConfig")
	}
	log.Info("Initialized CDI Config object")
	return configController, nil
}

// addConfigControllerWatches sets up the watches used by the config controller.
func addConfigControllerWatches(mgr manager.Manager, configController controller.Controller, cdiNamespace, configName, uploadProxyServiceName string, log logr.Logger) error {
	// Setup watches
	if err := watchCDIConfig(mgr, configController, configName); err != nil {
		return err
	}
	if err := watchStorageClass(mgr, configController, configName); err != nil {
		return err
	}
	if err := watchIngress(mgr, configController, cdiNamespace, configName, uploadProxyServiceName); err != nil {
		return err
	}
	if err := watchRoutes(mgr, configController, cdiNamespace, configName, uploadProxyServiceName); err != nil {
		return err
	}
	if err := watchClusterProxy(mgr, configController, configName); err != nil {
		return err
	}
	if err := watchUploadProxyCA(mgr, configController, configName); err != nil {
		return err
	}

	return nil
}

func watchCDIConfig(mgr manager.Manager, configController controller.Controller, configName string) error {
	if err := configController.Watch(source.Kind(mgr.GetCache(), &cdiv1.CDIConfig{}, &handler.TypedEnqueueRequestForObject[*cdiv1.CDIConfig]{})); err != nil {
		return err
	}
	return configController.Watch(source.Kind(mgr.GetCache(), &cdiv1.CDI{}, handler.TypedEnqueueRequestsFromMapFunc[*cdiv1.CDI](
		func(_ context.Context, _ *cdiv1.CDI) []reconcile.Request {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{Name: configName},
			}}
		},
	)))
}

func watchStorageClass(mgr manager.Manager, configController controller.Controller, configName string) error {
	return configController.Watch(source.Kind(mgr.GetCache(), &storagev1.StorageClass{}, handler.TypedEnqueueRequestsFromMapFunc[*storagev1.StorageClass](
		func(_ context.Context, _ *storagev1.StorageClass) []reconcile.Request {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{Name: configName},
			}}
		},
	)))
}

func watchIngress(mgr manager.Manager, configController controller.Controller, cdiNamespace, configName, uploadProxyServiceName string) error {
	err := configController.Watch(source.Kind(mgr.GetCache(), &networkingv1.Ingress{}, handler.TypedEnqueueRequestsFromMapFunc[*networkingv1.Ingress](
		func(_ context.Context, _ *networkingv1.Ingress) []reconcile.Request {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{Name: configName},
			}}
		}),
		predicate.TypedFuncs[*networkingv1.Ingress]{
			CreateFunc: func(e event.TypedCreateEvent[*networkingv1.Ingress]) bool {
				return "" != getURLFromIngress(e.Object, uploadProxyServiceName) &&
					e.Object.GetNamespace() == cdiNamespace
			},
			UpdateFunc: func(e event.TypedUpdateEvent[*networkingv1.Ingress]) bool {
				return "" != getURLFromIngress(e.ObjectNew, uploadProxyServiceName) &&
					e.ObjectNew.GetNamespace() == cdiNamespace
			},
			DeleteFunc: func(e event.TypedDeleteEvent[*networkingv1.Ingress]) bool {
				return "" != getURLFromIngress(e.Object, uploadProxyServiceName) &&
					e.Object.GetNamespace() == cdiNamespace
			},
		}))
	return err
}

// we only watch the route obj if they exist, i.e., if it is an OpenShift cluster
func watchRoutes(mgr manager.Manager, configController controller.Controller, cdiNamespace, configName, uploadProxyServiceName string) error {
	err := mgr.GetClient().List(context.TODO(), &routev1.RouteList{}, &client.ListOptions{Namespace: cdiNamespace})
	if !meta.IsNoMatchError(err) {
		if err == nil || cc.IsErrCacheNotStarted(err) {
			err := configController.Watch(source.Kind(mgr.GetCache(), &routev1.Route{}, handler.TypedEnqueueRequestsFromMapFunc[*routev1.Route](
				func(_ context.Context, _ *routev1.Route) []reconcile.Request {
					return []reconcile.Request{{
						NamespacedName: types.NamespacedName{Name: configName},
					}}
				}),
				predicate.TypedFuncs[*routev1.Route]{
					CreateFunc: func(e event.TypedCreateEvent[*routev1.Route]) bool {
						return "" != getURLFromRoute(e.Object, uploadProxyServiceName) &&
							e.Object.GetNamespace() == cdiNamespace
					},
					UpdateFunc: func(e event.TypedUpdateEvent[*routev1.Route]) bool {
						return "" != getURLFromRoute(e.ObjectNew, uploadProxyServiceName) &&
							e.ObjectNew.GetNamespace() == cdiNamespace
					},
					DeleteFunc: func(e event.TypedDeleteEvent[*routev1.Route]) bool {
						return "" != getURLFromRoute(e.Object, uploadProxyServiceName) &&
							e.Object.GetNamespace() == cdiNamespace
					},
				}))
			return err
		}
		return err
	}
	return nil
}

// we only watch the cluster-wide proxy obj if they exist, i.e., if it is an OpenShift cluster
func watchClusterProxy(mgr manager.Manager, configController controller.Controller, configName string) error {
	err := mgr.GetClient().List(context.TODO(), &ocpconfigv1.ProxyList{})
	if !meta.IsNoMatchError(err) {
		if err == nil || cc.IsErrCacheNotStarted(err) {
			return configController.Watch(source.Kind(mgr.GetCache(), &ocpconfigv1.Proxy{}, handler.TypedEnqueueRequestsFromMapFunc[*ocpconfigv1.Proxy](
				func(_ context.Context, _ *ocpconfigv1.Proxy) []reconcile.Request {
					return []reconcile.Request{{
						NamespacedName: types.NamespacedName{Name: configName},
					}}
				},
			)))
		}
		return err
	}
	return nil
}

// watchUploadProxyCA watches the kube-root-ca.crt ConfigMap for changes
// to the CA certificate used by the upload proxy.
//
// A change in the UploadProxyURL may invalidate the CA certificate, but
// watchCDIConfig will handle that.
func watchUploadProxyCA(mgr manager.Manager, configcontroller controller.Controller, configName string) error {
	handler := handler.TypedEnqueueRequestsFromMapFunc[*v1.ConfigMap](func(context.Context, *v1.ConfigMap) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: configName}}}
	})

	predicate := predicate.NewTypedPredicateFuncs[*v1.ConfigMap](func(o *v1.ConfigMap) bool {
		return o.Name == rootCertificateConfigMap
	})

	if err := configcontroller.Watch(source.Kind(mgr.GetCache(), &v1.ConfigMap{}, handler, predicate)); err != nil {
		return fmt.Errorf("could not watch UploadProxyCA ConfigMap: %w", err)
	}
	return nil
}

func getURLFromIngress(ing *networkingv1.Ingress, uploadProxyServiceName string) string {
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		if ing.Spec.DefaultBackend.Service.Name != uploadProxyServiceName {
			return ""
		}
		return ing.Spec.Rules[0].Host
	}
	for _, rule := range ing.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil && path.Backend.Service.Name == uploadProxyServiceName {
				if rule.Host != "" {
					return rule.Host
				}
			}
		}
	}
	return ""
}

func getURLFromRoute(route *routev1.Route, uploadProxyServiceName string) string {
	if route.Spec.To.Name == uploadProxyServiceName {
		if len(route.Status.Ingress) > 0 {
			return route.Status.Ingress[0].Host
		}
	}
	return ""
}

// getClusterWideProxy returns the OpenShift cluster wide proxy object
func getClusterWideProxy(r client.Client) (*ocpconfigv1.Proxy, error) {
	clusterWideProxy := &ocpconfigv1.Proxy{}
	// Ignore both no CRD found (IgnoreIsNoMatch) and the object itself not existing IsNotFound because we want to skip if not
	// in Open Shift.
	if err := r.Get(context.TODO(), types.NamespacedName{Name: ClusterWideProxyName}, clusterWideProxy); cc.IgnoreIsNoMatchError(err) != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}
	return clusterWideProxy, nil
}

// GetImportProxyConfig attempts to import proxy URLs if configured in the CDIConfig.
func GetImportProxyConfig(config *cdiv1.CDIConfig, field string) (string, error) {
	if config == nil {
		return "", errors.New("failed to get field, the CDIConfig is nil")
	}
	if config.Status.ImportProxy == nil {
		return "", errors.New("failed to get field, the CDIConfig ImportProxy is nil")
	}

	switch field {
	case common.ImportProxyHTTP:
		if config.Status.ImportProxy.HTTPProxy != nil {
			return *config.Status.ImportProxy.HTTPProxy, nil
		}
	case common.ImportProxyHTTPS:
		if config.Status.ImportProxy.HTTPSProxy != nil {
			return *config.Status.ImportProxy.HTTPSProxy, nil
		}
	case common.ImportProxyNoProxy:
		if config.Status.ImportProxy.NoProxy != nil {
			return *config.Status.ImportProxy.NoProxy, nil
		}
	case common.ImportProxyConfigMapName:
		if config.Status.ImportProxy.TrustedCAProxy != nil {
			return *config.Status.ImportProxy.TrustedCAProxy, nil
		}
	default:
		return "", errors.Errorf("CDIConfig ImportProxy does not have the field: %s", field)
	}

	// If everything fails, return blank
	return "", nil
}
