/*
Copyright 2020 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package importer

import (
	"context"
	"errors"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"k8s.io/klog"
)

const (
	destinationFile       = "/data/disk.img"
	nbdUnixSocket         = "/var/run/nbd.sock"
	nbdPidFile            = "/var/run/nbd.pid"
	nbdLibraryPath        = "/opt/vmware-vix-disklib-distrib/lib64"
	startupTimeoutSeconds = 60
)

// May be overridden in tests
var newVddkDataSource = createVddkDataSource
var vddkPluginPath = getVddkPluginPath

func getVddkPluginPath() string {
	mockPlugin := "/opt/testing/libvddk-test-plugin.so"
	_, err := os.Stat(mockPlugin)
	if !os.IsNotExist(err) {
		return mockPlugin
	}

	return "vddk"
}

// VDDKDataSource is the data provider for vddk.
// Currently just a reference to the nbdkit process.
type VDDKDataSource struct {
	Command   *exec.Cmd
	NbdSocket *url.URL
}

// FindMoRef takes the UUID of the VM to migrate and finds its MOref from the given VMware URL.
func FindMoRef(uuid string, sdkURL string) (string, error) {
	vmwURL, err := url.Parse(sdkURL)
	if err != nil {
		klog.Infof("Unable to create VMware URL: %s\n", err)
		return "", err
	}

	// Log in to vCenter
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := govmomi.NewClient(ctx, vmwURL, true)
	if err != nil {
		klog.Infof("Unable to connect to vCenter: %s\n", err)
		return "", err
	}
	defer conn.Logout(ctx)

	// Get the list of datacenters to search for VM UUID
	finder := find.NewFinder(conn.Client, true)
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		klog.Infof("Unable to retrieve datacenter list: %s\n", err)
		return "", err
	}

	// Search for VM matching given UUID, and save the MOref
	var moref string
	var instanceUUID bool
	searcher := object.NewSearchIndex(conn.Client)
	for _, datacenter := range datacenters {
		ref, err := searcher.FindByUuid(ctx, datacenter, uuid, true, &instanceUUID)
		if err != nil || ref == nil {
			klog.Infof("VM %s not found in datacenter %s.\n", uuid, datacenter)
		} else {
			moref = ref.Reference().Value
			klog.Infof("VM %s found in datacenter %s: %s\n", uuid, datacenter, moref)
		}
	}

	if moref == "" {
		return "", errors.New("unable to locate VM in any datacenter")
	}

	return moref, nil
}

// WaitForNbd waits for nbdkit to start by watching for the existence of the given PID file.
func WaitForNbd(pidfile string) error {
	nbdCheck := make(chan bool, 1)
	go func() {
		for {
			select {
			case <-nbdCheck:
				return
			case <-time.After(500 * time.Millisecond):
				klog.Infoln("Checking for nbdkit PID.")
				_, err := os.Stat(pidfile)
				if err != nil {
					klog.Infof("Error checking for nbdkit PID: %s\n", err)
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

// NewVDDKDataSource creates a new instance of the vddk data provider.
func NewVDDKDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string) (*VDDKDataSource, error) {
	return newVddkDataSource(endpoint, accessKey, secKey, thumbprint, uuid, backingFile)
}

func createVddkDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string) (*VDDKDataSource, error) {
	vmwURL, err := url.Parse(endpoint)
	if err != nil {
		klog.Infof("Unable to parse endpoint: %s\n", endpoint)
		return nil, err
	}

	// Construct VMware SDK URL and get MOref
	sdkURL := vmwURL.Scheme + "://" + accessKey + ":" + secKey + "@" + vmwURL.Host + "/sdk"
	moref, err := FindMoRef(uuid, sdkURL)
	if err != nil {
		return nil, err
	}

	args := []string{
		"--foreground",
		"--readonly",
		"--exit-with-parent",
		"--unix", nbdUnixSocket,
		"--pidfile", nbdPidFile,
		vddkPluginPath(),
		"server=" + vmwURL.Host,
		"user=" + accessKey,
		"password=" + secKey,
		"thumbprint=" + thumbprint,
		"vm=moref=" + moref,
		"file=" + backingFile,
		"libdir=" + nbdLibraryPath,
	}

	nbdkit := exec.Command("nbdkit", args...)
	env := os.Environ()
	env = append(env, "LD_LIBRARY_PATH="+nbdLibraryPath)
	nbdkit.Env = env

	stdout, err := nbdkit.StdoutPipe()
	if err != nil {
		klog.Infof("Error constructing stdout pipe: %s\n", err)
	}
	stderr, err := nbdkit.StderrPipe()
	if err != nil {
		klog.Infof("Error constructing stderr pipe: %s\n", err)
	}

	err = nbdkit.Start()
	if err != nil {
		klog.Infof("Unable to start nbdkit: %s\n", err)
		return nil, err
	}

	err = WaitForNbd(nbdPidFile)
	if err != nil {
		stdout, _ := ioutil.ReadAll(stdout)
		klog.Infof("stdout from nbdkit: %s\n", stdout)
		stderr, _ := ioutil.ReadAll(stderr)
		klog.Infof("stderr from nbdkit: %s\n", stderr)

		return nil, err
	}

	socket, _ := url.Parse("nbd://" + nbdUnixSocket)

	source := &VDDKDataSource{
		Command:   nbdkit,
		NbdSocket: socket,
	}
	return source, nil
}

// Info is called to get initial information about the data.
func (vs *VDDKDataSource) Info() (ProcessingPhase, error) {
	info, err := qemuOperations.Info(vs.GetURL())
	if err != nil {
		return ProcessingPhaseError, err
	}
	klog.Infof("qemu-img info: format %s, backing file %s, virtual size %d, actual size %d\n", info.Format, info.BackingFile, info.VirtualSize, info.ActualSize)
	return ProcessingPhaseTransferDataFile, nil
}

// Close closes any readers or other open resources.
func (vs *VDDKDataSource) Close() error {
	return vs.Command.Process.Kill()
}

// GetURL returns the url that the data processor can use when converting the data.
func (vs *VDDKDataSource) GetURL() *url.URL {
	return vs.NbdSocket
}

// Process is called to do any special processing before giving the url to the data back to the processor
func (vs *VDDKDataSource) Process() (ProcessingPhase, error) {
	return ProcessingPhaseTransferDataFile, nil
}

// Transfer is called to transfer the data from the source to the path passed in.
func (vs *VDDKDataSource) Transfer(path string) (ProcessingPhase, error) {
	return ProcessingPhaseTransferDataFile, nil
}

// TransferFile is called to transfer the data from the source to the file passed in.
func (vs *VDDKDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	err := qemuOperations.ConvertToRawStream(vs.GetURL(), destinationFile)
	if err != nil {
		klog.Infof("Failed to convert disk: %s\n", err)
		return ProcessingPhaseError, err
	}
	return ProcessingPhaseComplete, nil
}
