package main

import (
	"flag"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"

	populatormachinery "github.com/kubernetes-csi/lib-volume-populator/populator-machinery"
)

const (
	prefix     = "cdi.sample.populator"
	mountPath  = "/mnt"
	devicePath = "/dev/block"

	groupName  = "cdi.sample.populator"
	apiVersion = "v1alpha1"
	kind       = "SamplePopulator"
	resource   = "samplepopulators"

	controllerMode = "controller"
	populatorMode  = "populate"
)

var (
	mode         string
	fileName     string
	fileContents string
	httpEndpoint string
	metricsPath  string
	masterURL    string
	kubeconfig   string
	imageName    string
	namespace    string
)

type CDISamplePopulator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CDIPopulatorSpec `json:"spec"`
}

type CDIPopulatorSpec struct {
	FileName     string `json:"fileName"`
	FileContents string `json:"fileContents"`
}

func init() {
	klog.InitFlags(nil)

	// Main arg
	flag.StringVar(&mode, "mode", "", "(Mandatory) Mode to run in (controller, populate)")

	// Controller args
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&imageName, "image-name", "", "(Mandatory) Image to use for populating")
	flag.StringVar(&namespace, "namespace", "cdi", "Namespace to deploy controller")
	flag.StringVar(&httpEndpoint, "http-endpoint", "", "The TCP network address where the HTTP server for diagnostics, including metrics and leader election health check, will listen (example: `:8080`). The default is empty string, which means the server is disabled.")
	flag.StringVar(&metricsPath, "metrics-path", "/metrics", "The HTTP path where prometheus metrics will be exposed. Default is `/metrics`.")

	// Populator args
	flag.StringVar(&fileName, "file-name", "", "(Mandatory) File name to populate")
	flag.StringVar(&fileContents, "file-contents", "", "(Mandatory) Contents to populate file with")

	flag.Parse()
}

func getPopulatorPodArgs(rawBlock bool, u *unstructured.Unstructured) ([]string, error) {
	var samplePopulator CDISamplePopulator
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &samplePopulator)
	if nil != err {
		return nil, err
	}
	args := []string{"--mode=populate"}
	if rawBlock {
		args = append(args, "--file-name="+devicePath)
	} else {
		args = append(args, "--file-name="+mountPath+"/"+samplePopulator.Spec.FileName)
	}
	args = append(args, "--file-contents="+samplePopulator.Spec.FileContents)
	return args, nil
}

func runSampleController() {
	groupKind := schema.GroupKind{Group: groupName, Kind: kind}
	groupVersionResource := schema.GroupVersionResource{Group: groupName, Version: apiVersion, Resource: resource}

	// We run the default controller in populator-machinery, which will trigger this populator again in "populate" mode
	populatormachinery.RunController(masterURL, kubeconfig, imageName, httpEndpoint, metricsPath,
		namespace, prefix, groupKind, groupVersionResource, mountPath, devicePath, getPopulatorPodArgs)
}

func populatePVC(fileName, fileContents string) {
	f, err := os.Create(fileName)
	if nil != err {
		klog.Fatalf("Failed to create file: %v", err)
	}
	defer f.Close()

	if !strings.HasSuffix(fileContents, "\n") {
		fileContents += "\n"
	}

	_, err = f.WriteString(fileContents)
	if nil != err {
		klog.Fatalf("Failed to write to file: %v", err)
	}
}

func main() {
	klog.Infof("Initializing CDI sample populator in '%s' mode", mode)

	switch mode {
	case controllerMode:
		if imageName == "" {
			klog.Fatalf("Missing required arg")
		}
		runSampleController()
	case populatorMode:
		if fileName == "" || fileContents == "" {
			klog.Fatalf("Missing required arg")
		}
		populatePVC(fileName, fileContents)
	default:
		klog.Fatalf("Invalid mode: %s", mode)
	}

	klog.Infof("CDI sample populator finished succesfully in '%s' mode", mode)
}
