package image

import (
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
	"kubevirt.io/containerized-data-importer/pkg/system"
	"net/url"
	"strings"
)

var (
	nbdkitExecFunction = system.ExecWithLimits
)

type nbdkitOperations struct {
	nbdkit *Nbdkit
}

// NewNbdkitOperations return the implementation of nbdkit of QEMUOperations
func NewNbdkitOperations(n *Nbdkit) QEMUOperations {
	return &nbdkitOperations{nbdkit: n}
}

// NbdkitPlugin represents a plugin for nbdkit
type NbdkitPlugin string

// NbdkitFilter represents s filter for nbdkit
type NbdkitFilter string

// Nbdkit plugins
const (
	NbdkitCurlPlugin NbdkitPlugin = "curl"
)

// Nbdkit filters
const (
	NbdkitXzFilter   NbdkitFilter = "xz"
	NbdkitTarFilter  NbdkitFilter = "tar"
	NbdkitGzipFilter NbdkitFilter = "gzip"
)

// Nbdkit represents struct for an nbdkit instance
type Nbdkit struct {
	NbdPidFile string
	nbdkitArgs []string
	plugin     NbdkitPlugin
	pluginArgs []string
	filters    []NbdkitFilter
	source     *url.URL
}

// NewNbdkit creates a new Nbdkit instance with an nbdkit plugin and pid file
func NewNbdkit(plugin NbdkitPlugin, nbdkitPidFile string) *Nbdkit {
	return &Nbdkit{
		NbdPidFile: nbdkitPidFile,
		plugin:     plugin,
	}
}

// NewNbdkitCurl creates a new Nbdkit instance with the curl plugin
func NewNbdkitCurl(nbdkitPidFile, certDir string) *Nbdkit {
	var pluginArgs []string
	args := []string{"-r"}
	if certDir != "" {
		pluginArgs = append(pluginArgs, fmt.Sprintf("cainfo=%s/%s", certDir, "tls.crt"))
	}

	return &Nbdkit{
		NbdPidFile: nbdkitPidFile,
		plugin:     NbdkitCurlPlugin,
		nbdkitArgs: args,
		pluginArgs: pluginArgs,
	}
}

// AddFilter adds a nbdkit filter if it doesn't already exist
func (n *Nbdkit) AddFilter(filter NbdkitFilter) {
	for _, f := range n.filters {
		if f == filter {
			return
		}
	}
	n.filters = append(n.filters, filter)
}

// Info returns the information about the content provided with the url
func (n *nbdkitOperations) Info(url *url.URL) (*ImgInfo, error) {
	if len(url.Scheme) <= 0 {
		return Info(url)
	}
	n.nbdkit.source = url
	qemuImgArgs := []string{"--output=json"}
	output, err := n.nbdkit.startNbdkitWithQemuImg("info", qemuImgArgs)
	if err != nil {
		return nil, errors.Errorf("%s, %s", output, err.Error())
	}
	return checkOutputQemuImgInfo(output, url.String())
}

// Validate validates the url
func (n *nbdkitOperations) Validate(url *url.URL, availableSize int64, filesystemOverhead float64) error {
	info, err := n.Info(url)
	if err != nil {
		return err
	}
	return checkIfURLIsValid(info, availableSize, filesystemOverhead, url.String())
}

// ConvertToRawStream converts the content provided by the url to a raw disk in the dest
func (n *nbdkitOperations) ConvertToRawStream(url *url.URL, dest string, preallocate bool) error {
	if len(url.Scheme) <= 0 {
		return ConvertToRawStream(url, dest, preallocate)
	}
	n.nbdkit.source = url
	qemuImgArgs := []string{"-p", "-O", "raw", dest, "-t", "none"}
	if preallocate {
		klog.V(1).Info("Added preallocation")
		qemuImgArgs = append(qemuImgArgs, []string{"-o", "preallocation=falloc"}...)
	}
	_, err := n.nbdkit.startNbdkitWithQemuImg("convert", qemuImgArgs)
	return err
}

// CreateBlankImage creates empty raw image
func (n *nbdkitOperations) CreateBlankImage(dest string, size resource.Quantity, preallocate bool) error {
	// Use the default function to create an empty raw image
	return CreateBlankImage(dest, size, preallocate)
}

func (n *nbdkitOperations) Resize(image string, size resource.Quantity) error {
	return Resize(image, size)
}

func (n *Nbdkit) getSource() string {
	var source string
	switch n.plugin {
	case NbdkitCurlPlugin:
		source = fmt.Sprintf("url=%s", n.source.String())
	default:
		source = ""
	}
	return source
}

func (n *Nbdkit) startNbdkitWithQemuImg(qemuImgCmd string, qemuImgArgs []string) ([]byte, error) {
	argsNbdkit := []string{
		"--foreground",
		"--readonly",
		"-U", "-",
		"--pidfile", n.NbdPidFile,
	}
	// set filters
	for _, f := range n.filters {
		argsNbdkit = append(argsNbdkit, fmt.Sprintf("--filter=%s", f))
	}
	// set additional arguments
	for _, a := range n.nbdkitArgs {
		argsNbdkit = append(argsNbdkit, a)
	}
	// append nbdkit plugin arguments
	argsNbdkit = append(argsNbdkit, string(n.plugin), strings.Join(n.pluginArgs, " "), n.getSource())
	// append qemu-img command
	argsNbdkit = append(argsNbdkit, "--run", fmt.Sprintf("qemu-img %s $nbd %v", qemuImgCmd, strings.Join(qemuImgArgs, " ")))
	klog.V(3).Infof("Start nbdkit with: %v", argsNbdkit)
	return nbdkitExecFunction(nil, reportProgress, "nbdkit", argsNbdkit...)
}
