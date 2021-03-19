package image

import (
	"bufio"
	"fmt"
	"strings"

	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

const (
	nbdVddkLibraryPath    = "/opt/vmware-vix-disklib-distrib/lib64"
	startupTimeoutSeconds = 15
)

type nbdkitOperations struct {
	nbdkit *Nbdkit
}

// NbdkitPlugin represents a plugin for nbdkit
type NbdkitPlugin string

// NbdkitFilter represents s filter for nbdkit
type NbdkitFilter string

// Nbdkit plugins
const (
	NbdkitCurlPlugin     NbdkitPlugin = "curl"
	NbdkitFilePlugin     NbdkitPlugin = "file"
	NbdkitVddkPlugin     NbdkitPlugin = "vddk"
	NbdkitVddkMockPlugin NbdkitPlugin = "/opt/testing/libvddk-test-plugin.so"
)

// Nbdkit filters
const (
	NbdkitXzFilter    NbdkitFilter = "xz"
	NbdkitTarFilter   NbdkitFilter = "tar"
	NbdkitGzipFilter  NbdkitFilter = "gzip"
	NbdkitRetryFilter NbdkitFilter = "retry"
)

// Nbdkit represents struct for an nbdkit instance
type Nbdkit struct {
	c          *exec.Cmd
	NbdPidFile string
	nbdkitArgs []string
	plugin     NbdkitPlugin
	pluginArgs []string
	filters    []NbdkitFilter
	Socket     string
	Env        []string
}

// NbdkitOperation defines the interface for executing nbdkit
type NbdkitOperation interface {
	StartNbdkit(source string) error
	KillNbdkit() error
	AddEnvVariable(v string)
	AddFilter(filter NbdkitFilter)
}

// NewNbdkit creates a new Nbdkit instance with an nbdkit plugin and pid file
func NewNbdkit(plugin NbdkitPlugin, nbdkitPidFile string) *Nbdkit {
	return &Nbdkit{
		NbdPidFile: nbdkitPidFile,
		plugin:     plugin,
	}
}

// NewNbdkitCurl creates a new Nbdkit instance with the curl plugin
func NewNbdkitCurl(nbdkitPidFile, certDir, socket string) NbdkitOperation {
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
		Socket:     socket,
	}
}

// NewNbdkitVddk creates a new Nbdkit instance with the vddk plugin
func NewNbdkitVddk(nbdkitPidFile, socket, server, username, password, thumbprint, moref string) (NbdkitOperation, error) {

	pluginArgs := []string{
		"libdir=" + nbdVddkLibraryPath,
	}
	if server != "" {
		pluginArgs = append(pluginArgs, "server="+server)
	}
	if username != "" {
		pluginArgs = append(pluginArgs, "user="+username)
	}
	if password != "" {
		pluginArgs = append(pluginArgs, "password="+password)
	}
	if thumbprint != "" {
		pluginArgs = append(pluginArgs, "thumbprint="+thumbprint)
	}
	if moref != "" {
		pluginArgs = append(pluginArgs, "vm=moref="+moref)
	}
	p := getVddkPluginPath()
	n := &Nbdkit{
		NbdPidFile: nbdkitPidFile,
		plugin:     p,
		pluginArgs: pluginArgs,
		Socket:     socket,
	}

	n.AddEnvVariable("LD_LIBRARY_PATH=" + nbdVddkLibraryPath)
	n.AddFilter(NbdkitRetryFilter)
	if err := n.validatePlugin(); err != nil {
		return nil, err
	}
	return n, nil
}

// AddEnvVariable adds an environmental variable to the nbdkit command
func (n *Nbdkit) AddEnvVariable(v string) {
	env := os.Environ()
	env = append(env, v)
	n.Env = env
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

func getVddkPluginPath() NbdkitPlugin {
	_, err := os.Stat(string(NbdkitVddkMockPlugin))
	if !os.IsNotExist(err) {
		return NbdkitVddkMockPlugin
	}
	return NbdkitVddkPlugin
}

func (n *Nbdkit) getSourceArg(s string) string {
	var source string
	switch n.plugin {
	case NbdkitCurlPlugin:
		source = fmt.Sprintf("url=%s", s)
	case NbdkitVddkPlugin, NbdkitVddkMockPlugin:
		source = fmt.Sprintf("file=%s", s)
	default:
		source = s
	}
	return source
}

// StartNbdkit starts nbdkit process
func (n *Nbdkit) StartNbdkit(source string) error {
	var err error
	argsNbdkit := []string{
		"--foreground",
		"--readonly",
		"--exit-with-parent",
		"-U", n.Socket,
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
	argsNbdkit = append(argsNbdkit, string(n.plugin))
	argsNbdkit = append(argsNbdkit, n.pluginArgs...)
	argsNbdkit = append(argsNbdkit, n.getSourceArg(source))

	quotedArgs := make([]string, len(argsNbdkit))
	for index, value := range argsNbdkit {
		if strings.HasPrefix(value, "password=") {
			quotedArgs[index] = "'password=*****'"
		} else {
			quotedArgs[index] = "'" + value + "'"
		}
	}
	klog.V(3).Infof("Start nbdkit with: %v", quotedArgs)

	n.c = exec.Command("nbdkit", argsNbdkit...)
	var stdout io.ReadCloser
	stdout, err = n.c.StdoutPipe()
	if err != nil {
		klog.Errorf("Error constructing stdout pipe: %v", err)
		return err
	}
	n.c.Stderr = n.c.Stdout
	output := bufio.NewReader(stdout)
	go func() {
		for {
			line, err := output.ReadString('\n')
			if err != nil {
				break
			}
			klog.Infof("Log line from nbdkit: %s", line)
		}
		klog.Infof("Stopped watching nbdkit log.")
	}()

	err = n.c.Start()
	if err != nil {
		klog.Errorf("Unable to start nbdkit: %v", err)
		return err
	}

	err = waitForNbd(n.NbdPidFile)
	if err != nil {
		klog.Errorf("Failed waiting for nbdkit to start up: %v", err)
		return err
	}
	return nil
}

// waitForNbd waits for nbdkit to start by watching for the existence of the given PID file.
func waitForNbd(pidfile string) error {
	nbdCheck := make(chan bool, 1)
	go func() {
		klog.Infoln("Waiting for nbdkit PID.")
		for {
			select {
			case <-nbdCheck:
				return
			case <-time.After(500 * time.Millisecond):
				_, err := os.Stat(pidfile)
				if err != nil {
					if !os.IsNotExist(err) {
						klog.Warningf("Error checking for nbdkit PID: %v", err)
					}
				} else {
					nbdCheck <- true
					return
				}
			}
		}
	}()

	select {
	case <-nbdCheck:
		klog.Infoln("nbdkit ready.")
		return nil
	case <-time.After(startupTimeoutSeconds * time.Second):
		nbdCheck <- true
		return errors.New("timed out waiting for nbdkit to be ready")
	}
}

// KillNbdkit stops the nbdkit process
func (n *Nbdkit) KillNbdkit() error {
	if n.c == nil {
		return nil
	}
	if n.c.Process != nil {
		return n.c.Process.Signal(os.Interrupt)
	}
	return n.c.Process.Kill()
}

// validatePlugins tests VDDK and any other plugins before starting nbdkit for real
func (n *Nbdkit) validatePlugin() error {
	walker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		klog.Infof("%s: %d %s", path, info.Size(), info.Mode())
		return nil
	}

	klog.Infof("Checking nbdkit plugin directory tree:")
	err := filepath.Walk("/usr/lib64/nbdkit", walker)
	if err != nil {
		klog.Warningf("Unable to get nbdkit plugin directory tree: %v", err)
	}
	if n.plugin == NbdkitVddkPlugin {
		klog.Infof("Checking VDDK library directory tree:")
		err = filepath.Walk("/opt/vmware-vix-disklib-distrib", walker)
		if err != nil {
			klog.Warningf("Unable to get VDDK library directory tree: %v", err)
		}
	}
	args := []string{
		"--dump-plugin",
		string(n.plugin),
	}
	nbdkit := exec.Command("nbdkit", args...)
	nbdkit.Env = n.Env
	out, err := nbdkit.CombinedOutput()
	if out != nil {
		klog.Infof("Output from nbdkit --dump-plugin %s: %s", string(n.plugin), out)
	}
	if err != nil {
		return err
	}

	return nil
}

type mockNbdkit struct{}

// NewMockNbdkitCurl creates a mock nbdkit curl plugin for testing
func NewMockNbdkitCurl(nbdkitPidFile, certDir, socket string) NbdkitOperation {
	return &mockNbdkit{}
}

func (m *mockNbdkit) StartNbdkit(source string) error {
	return nil
}
func (m *mockNbdkit) KillNbdkit() error {
	return nil
}
func (m *mockNbdkit) AddEnvVariable(v string)       {}
func (m *mockNbdkit) AddFilter(filter NbdkitFilter) {}
