package main

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/kelseyhightower/envconfig"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	ocpconfigv1 "github.com/openshift/api/config/v1"
	imagev1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	forklift "kubevirt.io/containerized-data-importer-api/pkg/apis/forklift/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	dvc "kubevirt.io/containerized-data-importer/pkg/controller/datavolume"
	"kubevirt.io/containerized-data-importer/pkg/controller/populators"
	"kubevirt.io/containerized-data-importer/pkg/controller/transfer"
	metrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-controller"
	"kubevirt.io/containerized-data-importer/pkg/util"
	"kubevirt.io/containerized-data-importer/pkg/util/cert"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	"kubevirt.io/containerized-data-importer/pkg/util/cert/generator"
)

const (
	readyFile = "/tmp/ready"
)

var (
	kubeconfig             string
	kubeURL                string
	importerImage          string
	clonerImage            string
	uploadServerImage      string
	uploadProxyServiceName string
	ovirtPopulatorImage    string
	configName             string
	pullPolicy             string
	verbose                string
	installerLabels        map[string]string
	log                    = logf.Log.WithName("controller")
	controllerEnvs         ControllerEnvs
	resourcesSchemeFuncs   = []func(*apiruntime.Scheme) error{
		clientgoscheme.AddToScheme,
		cdiv1.AddToScheme,
		extv1.AddToScheme,
		snapshotv1.AddToScheme,
		forklift.AddToScheme,
		imagev1.Install,
		ocpconfigv1.Install,
		routev1.Install,
	}
)

// ControllerEnvs contains environment variables read for setting custom cert paths
type ControllerEnvs struct {
	UploadServerKeyFile           string `default:"/var/run/certs/cdi-uploadserver-signer/tls.key" split_words:"true"`
	UploadServerCertFile          string `default:"/var/run/certs/cdi-uploadserver-signer/tls.crt" split_words:"true"`
	UploadClientKeyFile           string `default:"/var/run/certs/cdi-uploadserver-client-signer/tls.key" split_words:"true"`
	UploadClientCertFile          string `default:"/var/run/certs/cdi-uploadserver-client-signer/tls.crt" split_words:"true"`
	UploadServerCaBundleConfigMap string `default:"cdi-uploadserver-signer-bundle" split_words:"true"`
	UploadClientCaBundleConfigMap string `default:"cdi-uploadserver-client-signer-bundle" split_words:"true"`
}

// The importer and cloner images are obtained here along with the supported flags. IMPORTER_IMAGE, CLONER_IMAGE, and UPLOADSERVICE_IMAGE
// are required by the controller and will cause it to fail if not defined.
// Note: kubeconfig hierarchy is 1) -kubeconfig flag, 2) $KUBECONFIG exported var. If neither is
// specified we do an in-cluster config. For testing it's easiest to export KUBECONFIG.
func init() {
	// flags
	flag.StringVar(&kubeURL, "server", "", "(Optional) URL address of a remote api server.  Do not set for local clusters.")
	klog.InitFlags(nil)
	flag.Parse()

	if flag.Lookup("kubeconfig") != nil {
		kubeconfig = flag.Lookup("kubeconfig").Value.String()
	}
	importerImage = getRequiredEnvVar("IMPORTER_IMAGE")
	clonerImage = getRequiredEnvVar("CLONER_IMAGE")
	uploadServerImage = getRequiredEnvVar("UPLOADSERVER_IMAGE")
	ovirtPopulatorImage = getRequiredEnvVar("OVIRT_POPULATOR_IMAGE")
	uploadProxyServiceName = getRequiredEnvVar("UPLOADPROXY_SERVICE")
	installerLabels = map[string]string{}

	pullPolicy = common.DefaultPullPolicy
	if pp := os.Getenv(common.PullPolicy); len(pp) != 0 {
		pullPolicy = pp
	}

	// We will need to put those on every resource our controller creates
	if partOfVal := os.Getenv(common.InstallerPartOfLabel); len(partOfVal) != 0 {
		installerLabels[common.AppKubernetesPartOfLabel] = partOfVal
	}
	if versionVal := os.Getenv(common.InstallerVersionLabel); len(versionVal) != 0 {
		installerLabels[common.AppKubernetesVersionLabel] = versionVal
	}

	configName = common.ConfigName

	// NOTE we used to have a constant here and we're now just passing in the level directly
	// that should be fine since it was a constant and not a mutable variable
	defVerbose := fmt.Sprintf("%d", 1) // note flag values are strings
	verbose = defVerbose
	// visit actual flags passed in and if passed check -v and set verbose
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "v" {
			verbose = f.Value.String()
		}
	})
	if verbose == defVerbose {
		klog.V(1).Infof("Note: increase the -v level in the controller deployment for more detailed logging, eg. -v=%d or -v=%d\n", 2, 3)
	}

	// Setup metrics for our various controllers
	if err := metrics.SetupMetrics(); err != nil {
		klog.Errorf("failed to setup metrics: %v", err)
		os.Exit(1)
	}

	klog.V(3).Infof("init: complete: cdi controller will create importer using image %q\n", importerImage)
}

func getRequiredEnvVar(name string) string {
	val := os.Getenv(name)
	if val == "" {
		klog.Fatalf("Environment Variable %q undefined\n", name)
	}
	return val
}

func start() {
	klog.Info("Starting CDI controller components")

	namespace := util.GetNamespace()

	cfg, err := clientcmd.BuildConfigFromFlags(kubeURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Unable to get kube config: %v\n", errors.WithStack(err))
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Unable to get kube client: %v\n", errors.WithStack(err))
	}

	// Setup scheme for all resources
	scheme := apiruntime.NewScheme()
	for _, f := range resourcesSchemeFuncs {
		err := f(scheme)
		if err != nil {
			klog.Errorf("Failed to add to scheme: %v", err)
			os.Exit(1)
		}
	}

	// client.New() returns a client without cache
	// since we don't have a cached client before manager init
	apiClient, err := client.New(cfg, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		klog.Fatalf("Unable to get uncached client: %v\n", errors.WithStack(err))
	}

	opts := manager.Options{
		LeaderElection:             true,
		LeaderElectionNamespace:    namespace,
		LeaderElectionID:           "cdi-controller-leader-election-helper",
		LeaderElectionResourceLock: "leases",
		Cache:                      getCacheOptions(apiClient, namespace),
		Scheme:                     scheme,
		Metrics: server.Options{
			BindAddress:   ":8443",
			SecureServing: true,
			// Disable HTTP/2 to prevent rapid reset vulnerability
			// See CVE-2023-44487, CVE-2023-39325
			TLSOpts: []func(*tls.Config){func(c *tls.Config) {
				c.NextProtos = []string{"http/1.1"}
			}},
		},
	}

	mgr, err := manager.New(config.GetConfigOrDie(), opts)
	if err != nil {
		klog.Errorf("Unable to setup controller manager: %v", err)
		os.Exit(1)
	}

	uploadClientCAFetcher := &fetcher.FileCertFetcher{KeyFileName: controllerEnvs.UploadClientKeyFile, CertFileName: controllerEnvs.UploadClientCertFile}
	uploadClientBundleFetcher := &fetcher.ConfigMapCertBundleFetcher{
		Name:   controllerEnvs.UploadClientCaBundleConfigMap,
		Client: k8sClient.CoreV1().ConfigMaps(namespace),
	}
	uploadClientCertGenerator := &generator.FetchCertGenerator{Fetcher: uploadClientCAFetcher}

	uploadServerCAFetcher := &fetcher.FileCertFetcher{KeyFileName: controllerEnvs.UploadServerKeyFile, CertFileName: controllerEnvs.UploadServerCertFile}
	uploadServerBundleFetcher := &fetcher.ConfigMapCertBundleFetcher{
		Name:   controllerEnvs.UploadServerCaBundleConfigMap,
		Client: k8sClient.CoreV1().ConfigMaps(namespace),
	}
	uploadServerCertGenerator := &generator.FetchCertGenerator{Fetcher: uploadServerCAFetcher}

	if _, err := controller.NewConfigController(mgr, log, uploadProxyServiceName, configName, installerLabels); err != nil {
		klog.Errorf("Unable to setup config controller: %v", err)
		os.Exit(1)
	}

	if _, err := controller.NewStorageProfileController(mgr, log, installerLabels); err != nil {
		klog.Errorf("Unable to setup storage profiles controller: %v", err)
		os.Exit(1)
	}

	if err := dvc.CreateCommonIndexes(mgr); err != nil {
		klog.Errorf("Unable to create shared indexes: %v", err)
		os.Exit(1)
	}

	ctx := signals.SetupSignalHandler()

	// TODO: Current DV controller had threadiness 3, should we do the same here, defaults to one thread.
	if _, err := dvc.NewImportController(ctx, mgr, log, installerLabels); err != nil {
		klog.Errorf("Unable to setup datavolume import controller: %v", err)
		os.Exit(1)
	}
	if _, err := dvc.NewUploadController(ctx, mgr, log, installerLabels); err != nil {
		klog.Errorf("Unable to setup datavolume upload controller: %v", err)
		os.Exit(1)
	}
	if _, err := dvc.NewPvcCloneController(ctx, mgr, log,
		clonerImage, importerImage, pullPolicy, getTokenPublicKey(), getTokenPrivateKey(), installerLabels); err != nil {
		klog.Errorf("Unable to setup datavolume pvc clone controller: %v", err)
		os.Exit(1)
	}
	if _, err := dvc.NewSnapshotCloneController(ctx, mgr, log,
		clonerImage, importerImage, pullPolicy, getTokenPublicKey(), getTokenPrivateKey(), installerLabels); err != nil {
		klog.Errorf("Unable to setup datavolume snapshot clone controller: %v", err)
		os.Exit(1)
	}
	if _, err := dvc.NewPopulatorController(ctx, mgr, log, installerLabels); err != nil {
		klog.Errorf("Unable to setup datavolume external-population controller: %v", err)
		os.Exit(1)
	}

	if _, err := controller.NewImportController(mgr, log, importerImage, pullPolicy, verbose, installerLabels); err != nil {
		klog.Errorf("Unable to setup import controller: %v", err)
		os.Exit(1)
	}

	if _, err := controller.NewCloneController(mgr, log, clonerImage, pullPolicy, verbose, uploadClientCertGenerator, uploadServerBundleFetcher, getTokenPublicKey(), installerLabels); err != nil {
		klog.Errorf("Unable to setup clone controller: %v", err)
		os.Exit(1)
	}

	if _, err := controller.NewUploadController(mgr, log, uploadServerImage, pullPolicy, verbose, uploadServerCertGenerator, uploadClientBundleFetcher, installerLabels); err != nil {
		klog.Errorf("Unable to setup upload controller: %v", err)
		os.Exit(1)
	}

	if _, err := transfer.NewObjectTransferController(mgr, log, installerLabels); err != nil {
		klog.Errorf("Unable to setup transfer controller: %v", err)
		os.Exit(1)
	}

	if _, err := controller.NewDataImportCronController(mgr, log, importerImage, pullPolicy, installerLabels); err != nil {
		klog.Errorf("Unable to setup dataimportcron controller: %v", err)
		os.Exit(1)
	}
	if _, err := controller.NewDataSourceController(mgr, log, installerLabels); err != nil {
		klog.Errorf("Unable to setup datasource controller: %v", err)
		os.Exit(1)
	}
	// Populator controllers and indexes
	if err := populators.CreateCommonPopulatorIndexes(mgr); err != nil {
		klog.Errorf("Unable to create common populator indexes: %v", err)
		os.Exit(1)
	}
	if _, err := populators.NewImportPopulator(ctx, mgr, log, installerLabels); err != nil {
		klog.Errorf("Unable to setup import populator: %v", err)
		os.Exit(1)
	}
	if _, err := populators.NewUploadPopulator(ctx, mgr, log, installerLabels); err != nil {
		klog.Errorf("Unable to setup upload populator: %v", err)
		os.Exit(1)
	}
	if _, err := populators.NewClonePopulator(ctx, mgr, log, clonerImage, pullPolicy, installerLabels, getTokenPublicKey()); err != nil {
		klog.Errorf("Unable to setup clone populator: %v", err)
		os.Exit(1)
	}
	if _, err := populators.NewForkliftPopulator(ctx, mgr, log, importerImage, ovirtPopulatorImage, installerLabels); err != nil {
		klog.Errorf("Unable to setup forklift populator: %v", err)
		os.Exit(1)
	}

	klog.V(1).Infoln("created cdi controllers")

	if err := mgr.Start(ctx); err != nil {
		klog.Errorf("Error running manager: %v", err)
		os.Exit(1)
	}
}

func main() {
	defer klog.Flush()
	debug := false
	verbosityLevel, err := strconv.Atoi(verbose)
	if err == nil && verbosityLevel > 1 {
		debug = true
	}
	err = envconfig.Process("", &controllerEnvs)
	if err != nil {
		klog.Fatalf("Unable to get environment variables: %v\n", errors.WithStack(err))
	}

	logf.SetLogger(zap.New(zap.Level(zapcore.Level(-1*verbosityLevel)), zap.UseDevMode(debug)))
	logf.Log.WithName("main").Info("Verbosity level", "verbose", verbose, "debug", debug)

	if err = createReadyFile(); err != nil {
		klog.Fatalf("Error creating ready file: %+v", err)
	}

	start()

	deleteReadyFile()

	klog.V(2).Infoln("cdi controller exited")
}

func createReadyFile() error {
	f, err := os.Create(readyFile)
	if err != nil {
		return err
	}
	defer f.Close()
	return nil
}

func deleteReadyFile() {
	os.Remove(readyFile)
}

func getTokenPublicKey() *rsa.PublicKey {
	keyBytes, err := os.ReadFile(controller.TokenPublicKeyPath)
	if err != nil {
		klog.Fatalf("Error reading apiserver public key")
	}

	key, err := controller.DecodePublicKey(keyBytes)
	if err != nil {
		klog.Fatalf("Error decoding public key")
	}

	return key
}

func getTokenPrivateKey() *rsa.PrivateKey {
	bytes, err := os.ReadFile(controller.TokenPrivateKeyPath)
	if err != nil {
		klog.Fatalf("Error reading private key")
	}

	obj, err := cert.ParsePrivateKeyPEM(bytes)
	if err != nil {
		klog.Fatalf("Error decoding private key")
	}

	key, ok := obj.(*rsa.PrivateKey)
	if !ok {
		klog.Fatalf("Invalid private key format")
	}

	return key
}

// Restricts some types in the cache's ListWatch to specific fields/labels per GVK at the specified object,
// other types will continue working normally.
// Note: objects you read once with the controller runtime client are cached.
// TODO: Make our watches way more specific using labels, for example,
// at the point of writing this, we don't care about VolumeSnapshots without the CDI label
func getCacheOptions(apiClient client.Client, cdiNamespace string) cache.Options {
	namespaceSelector := fields.Set{"metadata.namespace": cdiNamespace}.AsSelector()

	cacheOptions := cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&networkingv1.Ingress{}: {
				Field: namespaceSelector,
			},
			&batchv1.CronJob{}: {
				Field: namespaceSelector,
			},
			&batchv1.Job{}: {
				Field: namespaceSelector,
			},
			&v1.ConfigMap{}: {
				Field: namespaceSelector,
			},
			&v1.Secret{}: {
				Field: namespaceSelector,
			},
		},
	}

	cacheOptionsByObjectForOpenshift := map[client.Object]cache.ByObject{
		&routev1.Route{}: {
			Field: namespaceSelector,
		},
	}

	// Currently controller-runtime will fail if types in here are not installed in the cluster
	// https://github.com/kubernetes-sigs/controller-runtime/issues/2456
	if isOpenShift(apiClient) {
		for k, v := range cacheOptionsByObjectForOpenshift {
			cacheOptions.ByObject[k] = v
		}
	}

	return cacheOptions
}

func isOpenShift(apiClient client.Client) bool {
	clusterVersion := &ocpconfigv1.ClusterVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "version",
		},
	}
	if err := apiClient.Get(context.TODO(), client.ObjectKeyFromObject(clusterVersion), clusterVersion); err != nil {
		if !meta.IsNoMatchError(err) {
			klog.Errorf("Error getting clusterVersion: %v", err)
			os.Exit(1)
		}
		return false
	}

	return true
}
