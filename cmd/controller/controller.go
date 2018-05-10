package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/golang/glog"
	. "github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/kubevirt/containerized-data-importer/pkg/controller"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	configPath    string
	masterURL     string
	importerImage string
	pullPolicy    string
	verbose       string
)

// The optional importer image is obtained here along with the supported flags.
// Note: kubeconfig hierarchy is 1) -kubeconfig flag, 2) $KUBECONFIG exported var. If neither is
//   specified we do an in-cluster config. For testing it's easiest to export KUBECONFIG.
func init() {
	// optional, importer image.  If not provided, uses IMPORTER_DEFAULT_IMAGE
	const IMPORTER_IMAGE = "IMPORTER_IMAGE"

	// flags
	flag.StringVar(&configPath, "kubeconfig", os.Getenv("KUBECONFIG"), "(Optional) Overrides $KUBECONFIG")
	flag.StringVar(&masterURL, "server", "", "(Optional) URL address of a remote api server.  Do not set for local clusters.")
	flag.Parse()

	// env variables
	importerImage = os.Getenv(IMPORTER_IMAGE)
	if importerImage == "" {
		importerImage = IMPORTER_DEFAULT_IMAGE
	}
	pullPolicy = IMPORTER_DEFAULT_PULL_POLICY
	if pp := os.Getenv(IMPORTER_PULL_POLICY); len(pp) != 0 {
		pullPolicy = pp
	}

	// get the verbose level so it can be passed to the importer pod
	defVerbose := fmt.Sprintf("%d", IMPORTER_DEFAULT_VERBOSE) // note flag values are strings
	verbose = defVerbose
	// visit actual flags passed in and if passed check -v and set verbose
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "v" {
			verbose = f.Value.String()
		}
	})
	if verbose == defVerbose {
		glog.V(Vuser).Infof("Note: increase the -v level in the controller deployment for more detailed logging, eg. -v=%d or -v=%d\n", Vadmin, Vdebug)
	}

	glog.V(Vdebug).Infof("init: complete: cdi controller will create importer using image %q\n", importerImage)
}

func main() {
	defer glog.Flush()

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, configPath)
	if err != nil {
		glog.Fatalf("Unable to get kube config: %v\n", errors.WithStack(err))
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Unable to get kube client: %v\n", errors.WithStack(err))
	}

	pvcInformerFactory := informers.NewSharedInformerFactory(client, DEFAULT_RESYNC_PERIOD)
	podInformerFactory := informers.NewFilteredSharedInformerFactory(client, DEFAULT_RESYNC_PERIOD, "", func(options *v1.ListOptions) {
		options.LabelSelector = CDI_LABEL_SELECTOR
	})

	pvcInformer := pvcInformerFactory.Core().V1().PersistentVolumeClaims().Informer()
	podInformer := podInformerFactory.Core().V1().Pods().Informer()

	cdiController := controller.NewController(client, pvcInformer, podInformer, importerImage, pullPolicy, verbose)
	glog.V(Vuser).Infoln("created cdi controller")
	stopCh := handleSignals()
	err = cdiController.Run(1, stopCh)
	if err != nil {
		glog.Fatalln("Error running controller: %+v", err)
	}
	glog.V(Vadmin).Infoln("cdi controller exited")
}

// Shutdown gracefully on system signals
func handleSignals() <-chan struct{} {
	sigCh := make(chan os.Signal)
	stopCh := make(chan struct{})
	go func() {
		signal.Notify(sigCh)
		<-sigCh
		close(stopCh)
		os.Exit(1)
	}()
	return stopCh
}
