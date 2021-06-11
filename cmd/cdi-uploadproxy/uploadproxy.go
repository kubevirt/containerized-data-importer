package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"kubevirt.io/containerized-data-importer/pkg/uploadproxy"
	"kubevirt.io/containerized-data-importer/pkg/util"
	certfetcher "kubevirt.io/containerized-data-importer/pkg/util/cert/fetcher"
	certwatcher "kubevirt.io/containerized-data-importer/pkg/util/cert/watcher"
)

const (
	// Default port that api listens on.
	defaultPort = 8443

	// Default address api listens on.
	defaultHost = "0.0.0.0"

)

var (
	configPath      string
	masterURL       string
	verbose         string
	uploadProxyEnvs UploadProxyEnvs
)

// UploadProxyEnvs contains environment variables read for setting custom cert paths
type UploadProxyEnvs struct {
	ServerCertFile                string `default:"/var/run/certs/cdi-uploadproxy-server-cert/tls.crt" split_words:"true"`
	ServerKeyFile                 string `default:"/var/run/certs/cdi-uploadproxy-server-cert/tls.key" split_words:"true"`
	UploadClientKeyFile           string `default:"/var/run/certs/cdi-uploadserver-client-cert/tls.key" split_words:"true"`
	UploadClientCertFile          string `default:"/var/run/certs/cdi-uploadserver-client-cert/tls.crt" split_words:"true"`
	UploadServerCABundleConfigMap string `default:"cdi-uploadserver-signer-bundle" split_words:"true"`
}

func init() {
	// flags
	flag.StringVar(&configPath, "kubeconfig", os.Getenv("KUBECONFIG"), "(Optional) Overrides $KUBECONFIG")
	flag.StringVar(&masterURL, "server", "", "(Optional) URL address of a remote api server.  Do not set for local clusters.")
	klog.InitFlags(nil)
	flag.Parse()

	// get the verbose level so it can be passed to the importer pod
	defVerbose := fmt.Sprintf("%d", 1) // note flag values are strings
	verbose = defVerbose
	// visit actual flags passed in and if passed check -v and set verbose
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "v" {
			verbose = f.Value.String()
		}
	})
	if verbose == defVerbose {
		klog.V(1).Infof("Note: increase the -v level in the api deployment for more detailed logging, eg. -v=%d or -v=%d\n", 2, 3)
	}
}

func main() {
	defer klog.Flush()

	namespace := util.GetNamespace()

	err := envconfig.Process("", &uploadProxyEnvs)
	if err != nil {
		klog.Fatalf("Unable to get environment variables: %v\n", errors.WithStack(err))
	}

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, configPath)
	if err != nil {
		klog.Fatalf("Unable to get kube config: %v\n", errors.WithStack(err))
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Unable to get kube client: %v\n", errors.WithStack(err))
	}
	apiServerPublicKey, err := getAPIServerPublicKey()
	if err != nil {
		klog.Fatalf("Unable to get apiserver public key %v\n", errors.WithStack(err))
	}
	certWatcher, err := certwatcher.New(uploadProxyEnvs.ServerCertFile, uploadProxyEnvs.ServerKeyFile)
	if err != nil {
		klog.Fatalf("Unable to create certwatcher: %v\n", errors.WithStack(err))
	}

	clientCertFetcher := &certfetcher.FileCertFetcher{KeyFileName: uploadProxyEnvs.UploadClientKeyFile, CertFileName: uploadProxyEnvs.UploadClientCertFile}
	serverCAFetcher := &certfetcher.ConfigMapCertBundleFetcher{
		Name:   uploadProxyEnvs.UploadServerCABundleConfigMap,
		Client: client.CoreV1().ConfigMaps(namespace),
	}

	uploadProxy, err := uploadproxy.NewUploadProxy(defaultHost,
		defaultPort,
		apiServerPublicKey,
		certWatcher,
		clientCertFetcher,
		serverCAFetcher,
		client)
	if err != nil {
		klog.Fatalf("UploadProxy failed to initialize: %v\n", errors.WithStack(err))
	}

	go certWatcher.Start(signals.SetupSignalHandler())

	err = uploadProxy.Start()
	if err != nil {
		klog.Fatalf("TLS server failed: %v\n", errors.WithStack(err))
	}
}

func getAPIServerPublicKey() (string, error) {
	const envName = "APISERVER_PUBLIC_KEY"
	val, ok := os.LookupEnv(envName)
	if !ok {
		return "", errors.Errorf("%s not defined", envName)
	}
	return val, nil
}
