package main

import (
	"flag"
	"github.com/golang/glog"
	"github.com/kubevirt/containerized-data-importer/pkg/controller"
	"github.com/kubevirt/containerized-data-importer/pkg/util"
	"k8s.io/client-go/kubernetes"
	"os"
	"path/filepath"
)

var (
	configPath string
	masterURL  string
	kubeEnvVar string
)

func init() {
	home := os.Getenv("HOME")
	kubeEnvVar = os.Getenv("KUBERNETES_PORT") // KUBERNETES_PORT is always set in a pod environment
	flag.StringVar(&configPath, "kubeconfig", filepath.Join(home, ".kube", "config"), "(Optional) Absolute path to kubeconfig. (Default: $HOME/.kube/config)")
	flag.StringVar(&masterURL, "server", "", "(Optional) URL address of api server (Default: localhost:443)")
	flag.Parse()
	glog.Infoln("CDI Controller is initialized.")
}

func main() {
	var client kubernetes.Interface
	if kubeEnvVar != "" {
		glog.Infoln("Detected in-cluster environment")
		client = util.GetInClusterClient()
	} else {
		glog.Infoln("Deteced out-of-cluster environment")
		client = util.GetOutOfClusterClient(configPath, masterURL)
	}
	_ = controller.NewController(client)

}
