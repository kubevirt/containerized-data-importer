package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	. "kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/uploadproxy"
)

const (
	// Default port that api listens on.
	defaultPort = 8443

	// Default address api listens on.
	defaultHost = "0.0.0.0"
)

var (
	configPath string
	masterURL  string
	verbose    string
)

func init() {
	// flags
	flag.StringVar(&configPath, "kubeconfig", os.Getenv("KUBECONFIG"), "(Optional) Overrides $KUBECONFIG")
	flag.StringVar(&masterURL, "server", "", "(Optional) URL address of a remote api server.  Do not set for local clusters.")
	flag.Parse()

	// get the verbose level so it can be passed to the importer pod
	defVerbose := fmt.Sprintf("%d", DEFAULT_VERBOSE) // note flag values are strings
	verbose = defVerbose
	// visit actual flags passed in and if passed check -v and set verbose
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "v" {
			verbose = f.Value.String()
		}
	})
	if verbose == defVerbose {
		glog.V(Vuser).Infof("Note: increase the -v level in the api deployment for more detailed logging, eg. -v=%d or -v=%d\n", Vadmin, Vdebug)
	}
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

	uploadProxy, err := uploadproxy.NewUploadProxy(defaultHost, defaultPort, client)
	if err != nil {
		glog.Fatalf("Upload upload proxy failed to initialize: %v\n", errors.WithStack(err))
	}

	err = uploadProxy.Start()
	if err != nil {
		glog.Fatalf("TLS server failed: %v\n", errors.WithStack(err))
	}
}
