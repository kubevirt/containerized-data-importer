package main

import (
	"flag"
	"os"
	"os/signal"

	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/common"
	"github.com/kubevirt/containerized-data-importer/pkg/controller"
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
		importerImage = common.IMPORTER_DEFAULT_IMAGE
	}
	pullPolicy = common.IMPORTER_DEFAULT_PULL_POLICY
	if pp := os.Getenv(common.IMPORTER_PULL_POLICY); len(pp) != 0 {
		pullPolicy = pp
	}
	glog.Infof("init: complete: CDI controller will create the %q version of the importer\n", importerImage)
}

func main() {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, configPath)
	if err != nil {
		glog.Fatalf("Error getting kube config: %v\n", err)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error getting kube client: %v\n", err)
	}

	pvcInformerFactory := informers.NewSharedInformerFactory(client, common.DEFAULT_RESYNC_PERIOD)
	podInformerFactory := informers.NewFilteredSharedInformerFactory(client, common.DEFAULT_RESYNC_PERIOD, "", func(options *v1.ListOptions) {
		options.LabelSelector = common.CDI_LABEL_SELECTOR
	})

	pvcInformer := pvcInformerFactory.Core().V1().PersistentVolumeClaims().Informer()
	podInformer := podInformerFactory.Core().V1().Pods().Informer()

	cdiController, err := controller.NewController(client, pvcInformer, podInformer, importerImage, pullPolicy)
	if err != nil {
		glog.Fatal("Error creating CDI controller: %v", err)
	}
	glog.Infoln("main: created CDI Controller")
	stopCh := handleSignals()
	err = cdiController.Run(1, stopCh)
	if err != nil {
		glog.Fatalln(err)
	}
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
