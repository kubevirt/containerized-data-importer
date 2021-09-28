package main

import (
	"context"
	"crypto/rsa"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/kelseyhightower/envconfig"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	"github.com/pkg/errors"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	extclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	crdinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	"kubevirt.io/containerized-data-importer/pkg/controller/transfer"
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
	configName             string
	pullPolicy             string
	verbose                string
	installerLabels        map[string]string
	log                    = logf.Log.WithName("controller")
	controllerEnvs         ControllerEnvs
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
//   specified we do an in-cluster config. For testing it's easiest to export KUBECONFIG.
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

	klog.V(3).Infof("init: complete: cdi controller will create importer using image %q\n", importerImage)
}

func getRequiredEnvVar(name string) string {
	val := os.Getenv(name)
	if val == "" {
		klog.Fatalf("Environment Variable %q undefined\n", name)
	}
	return val
}

func start(ctx context.Context, cfg *rest.Config) {
	klog.Info("Starting CDI controller components")

	namespace := util.GetNamespace()

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Unable to get kube client: %v\n", errors.WithStack(err))
	}

	extClient, err := extclientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building extClient: %s", err.Error())
	}

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	if err != nil {
		klog.Errorf("Unable to setup controller manager: %v", err)
		os.Exit(1)
	}

	crdInformerFactory := crdinformers.NewSharedInformerFactory(extClient, common.DefaultResyncPeriod)
	crdInformer := crdInformerFactory.Apiextensions().V1().CustomResourceDefinitions().Informer()

	uploadClientCAFetcher := &fetcher.FileCertFetcher{KeyFileName: controllerEnvs.UploadClientKeyFile, CertFileName: controllerEnvs.UploadClientCertFile}
	uploadClientBundleFetcher := &fetcher.ConfigMapCertBundleFetcher{
		Name:   controllerEnvs.UploadClientCaBundleConfigMap,
		Client: client.CoreV1().ConfigMaps(namespace),
	}
	uploadClientCertGenerator := &generator.FetchCertGenerator{Fetcher: uploadClientCAFetcher}

	uploadServerCAFetcher := &fetcher.FileCertFetcher{KeyFileName: controllerEnvs.UploadServerKeyFile, CertFileName: controllerEnvs.UploadServerCertFile}
	uploadServerBundleFetcher := &fetcher.ConfigMapCertBundleFetcher{
		Name:   controllerEnvs.UploadServerCaBundleConfigMap,
		Client: client.CoreV1().ConfigMaps(namespace),
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

	// TODO: Current DV controller had threadiness 3, should we do the same here, defaults to one thread.
	if _, err := controller.NewDatavolumeController(mgr, extClient, log,
		clonerImage, pullPolicy, getTokenPublicKey(), getTokenPrivateKey(), installerLabels); err != nil {
		klog.Errorf("Unable to setup datavolume controller: %v", err)
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

	klog.V(1).Infoln("created cdi controllers")

	go crdInformerFactory.Start(ctx.Done())

	// Add Crd informer, so we can start the smart clone controller if we detect the CSI CRDs being installed.
	addCrdInformerEventHandlers(crdInformer, extClient, mgr, log)

	if err := mgr.Start(ctx); err != nil {
		klog.Errorf("Error running manager: %v", err)
		os.Exit(1)
	}
}

func main() {
	defer klog.Flush()
	debug := false
	if i, err := strconv.Atoi(verbose); err == nil && i > 1 {
		debug = true
	}
	err := envconfig.Process("", &controllerEnvs)
	if err != nil {
		klog.Fatalf("Unable to get environment variables: %v\n", errors.WithStack(err))
	}

	logf.SetLogger(zap.New(zap.UseDevMode(debug)))
	logf.Log.WithName("main").Info("Verbosity level", "verbose", verbose, "debug", debug)

	cfg, err := clientcmd.BuildConfigFromFlags(kubeURL, kubeconfig)
	if err != nil {
		klog.Fatalf("Unable to get kube config: %v\n", errors.WithStack(err))
	}

	ctx := signals.SetupSignalHandler()

	err = startLeaderElection(context.TODO(), cfg, func() {
		start(ctx, cfg)
	})

	if err != nil {
		klog.Fatalf("Unable to start leader election: %v\n", errors.WithStack(err))
	}

	if err = createReadyFile(); err != nil {
		klog.Fatalf("Error creating ready file: %+v", err)
	}

	<-ctx.Done()

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

func addCrdInformerEventHandlers(crdInformer cache.SharedIndexInformer, extclient extclientset.Interface, mgr manager.Manager, log logr.Logger) {
	crdInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			crd := obj.(*extv1.CustomResourceDefinition)
			crdName := crd.Name

			vs := "volumesnapshots." + snapshotv1.GroupName

			switch crdName {
			case vs:
				startSmartController(extclient, mgr, log)
			}
		},
	})
}

func startSmartController(extclient extclientset.Interface, mgr manager.Manager, log logr.Logger) {
	if controller.IsCsiCrdsDeployed(extclient) {
		log.Info("CSI CRDs detected, starting smart clone controller")
		if _, err := controller.NewSmartCloneController(mgr, log, installerLabels); err != nil {
			log.Error(err, "Unable to setup smart clone controller: %v")
		}
	}
}

func getTokenPublicKey() *rsa.PublicKey {
	keyBytes, err := ioutil.ReadFile(controller.TokenPublicKeyPath)
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
	bytes, err := ioutil.ReadFile(controller.TokenPrivateKeyPath)
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
