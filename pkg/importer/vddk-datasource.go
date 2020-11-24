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
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	libnbd "github.com/mrnold/go-libnbd"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"golang.org/x/sys/unix"
	"k8s.io/klog/v2"
	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
)

const (
	destinationFile       = "/data/disk.img"
	nbdUnixSocket         = "/var/run/nbd.sock"
	nbdPidFile            = "/var/run/nbd.pid"
	nbdLibraryPath        = "/opt/vmware-vix-disklib-distrib/lib64"
	startupTimeoutSeconds = 15
)

// May be overridden in tests
var newVddkDataSource = createVddkDataSource
var newVddkDataSink = createVddkDataSink
var vddkPluginPath = getVddkPluginPath

func getVddkPluginPath() string {
	mockPlugin := "/opt/testing/libvddk-test-plugin.so"
	_, err := os.Stat(mockPlugin)
	if !os.IsNotExist(err) {
		return mockPlugin
	}

	return "vddk"
}

// NbdOperations provides a mockable interface for the things needed from libnbd.
type NbdOperations interface {
	GetSize() (uint64, error)
	Pread(buf []byte, offset uint64, optargs *libnbd.PreadOptargs) error
	Close() *libnbd.LibnbdError
}

// VDDKDataSink provides a mockable interface for saving data from the source.
type VDDKDataSink interface {
	Write(buf []byte) (int, error)
	Close()
}

// VDDKFileSink writes the source disk data to a local file.
type VDDKFileSink struct {
	file   *os.File
	writer *bufio.Writer
}

func (sink *VDDKFileSink) Write(buf []byte) (int, error) {
	written, err := sink.writer.Write(buf)
	if err != nil {
		return written, err
	}
	err = sink.writer.Flush()
	return written, err
}

// Close closes the file after a transfer is complete.
func (sink *VDDKFileSink) Close() {
	sink.writer.Flush()
	sink.file.Sync()
	sink.file.Close()
}

// VDDKDataSource is the data provider for vddk.
type VDDKDataSource struct {
	Command   *exec.Cmd
	NbdSocket *url.URL
	NbdHandle NbdOperations
}

func init() {
	if err := prometheus.Register(progress); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			// A counter for that metric has been registered before.
			// Use the old counter from now on.
			progress = are.ExistingCollector.(*prometheus.CounterVec)
		} else {
			klog.Errorf("Unable to create prometheus progress counter")
		}
	}
	ownerUID, _ = util.ParseEnvVar(common.OwnerUID, false)
}

// FindMoRef takes the UUID of the VM to migrate and finds its MOref from the given VMware URL.
func FindMoRef(uuid string, sdkURL string) (string, error) {
	vmwURL, err := url.Parse(sdkURL)
	if err != nil {
		klog.Errorf("Unable to create VMware URL: %v", err)
		return "", err
	}

	// Log in to vCenter
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := govmomi.NewClient(ctx, vmwURL, true)
	if err != nil {
		klog.Errorf("Unable to connect to vCenter: %v", err)
		return "", err
	}
	defer conn.Logout(ctx)

	// Get the list of datacenters to search for VM UUID
	finder := find.NewFinder(conn.Client, true)
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		klog.Errorf("Unable to retrieve datacenter list: %v", err)
		return "", err
	}

	// Search for VM matching given UUID, and save the MOref
	var moref string
	var instanceUUID bool
	searcher := object.NewSearchIndex(conn.Client)
	for _, datacenter := range datacenters {
		ref, err := searcher.FindByUuid(ctx, datacenter, uuid, true, &instanceUUID)
		if err != nil || ref == nil {
			klog.Infof("VM %s not found in datacenter %s.", uuid, datacenter)
		} else {
			moref = ref.Reference().Value
			klog.Infof("VM %s found in datacenter %s: %s", uuid, datacenter, moref)
			vm := object.NewVirtualMachine(conn.Client, ref.Reference())
			state, err := vm.PowerState(ctx)
			if err != nil {
				klog.Warningf("Unable to get current VM power state: %v", err)
			} else {
				klog.Infof("Current VM power state: %s", state)
			}
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

// NewVDDKDataSource creates a new instance of the vddk data provider.
func NewVDDKDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string, currentCheckpoint string, previousCheckpoint string, finalCheckpoint string) (*VDDKDataSource, error) {
	return newVddkDataSource(endpoint, accessKey, secKey, thumbprint, uuid, backingFile, currentCheckpoint, previousCheckpoint, finalCheckpoint)
}

func validatePlugins() error {
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

	klog.Infof("Checking VDDK library directory tree:")
	err = filepath.Walk("/opt/vmware-vix-disklib-distrib", walker)
	if err != nil {
		klog.Warningf("Unable to get VDDK library directory tree: %v", err)
	}

	args := []string{
		"--dump-plugin",
		vddkPluginPath(),
	}
	nbdkit := exec.Command("nbdkit", args...)
	env := os.Environ()
	env = append(env, "LD_LIBRARY_PATH="+nbdLibraryPath)
	nbdkit.Env = env
	out, err := nbdkit.CombinedOutput()
	if out != nil {
		klog.Infof("Output from nbdkit --dump-plugin %s: %s", vddkPluginPath(), out)
	}
	if err != nil {
		return err
	}

	return nil
}

func createVddkDataSource(endpoint string, accessKey string, secKey string, thumbprint string, uuid string, backingFile string, currentCheckpoint string, previousCheckpoint string, finalCheckpoint string) (*VDDKDataSource, error) {
	klog.Infof("Checkpoints: %s %s %s", currentCheckpoint, previousCheckpoint, finalCheckpoint)
	vmwURL, err := url.Parse(endpoint)
	if err != nil {
		klog.Errorf("Unable to parse endpoint: %v", endpoint)
		return nil, err
	}

	// Construct VMware SDK URL and get MOref
	sdkURL := vmwURL.Scheme + "://" + accessKey + ":" + secKey + "@" + vmwURL.Host + "/sdk"
	moref, err := FindMoRef(uuid, sdkURL)
	if err != nil {
		return nil, err
	}

	err = validatePlugins()
	if err != nil {
		klog.Errorf("Error validating nbdkit plugins: %v", err)
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
		klog.Errorf("Error constructing stdout pipe: %v", err)
		return nil, err
	}
	nbdkit.Stderr = nbdkit.Stdout
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

	err = nbdkit.Start()
	if err != nil {
		klog.Errorf("Unable to start nbdkit: %v", err)
		return nil, err
	}

	err = WaitForNbd(nbdPidFile)
	if err != nil {
		klog.Errorf("Failed waiting for nbdkit to start up: %v", err)
		return nil, err
	}

	handle, err := libnbd.Create()
	if err != nil {
		klog.Errorf("Unable to create libnbd handle: %v", err)
		nbdkit.Process.Kill()
		return nil, err
	}

	socket, _ := url.Parse("nbd://" + nbdUnixSocket)
	err = handle.ConnectUri("nbd+unix://?socket=" + nbdUnixSocket)
	if err != nil {
		klog.Errorf("Unable to connect to socket %s: %v", socket, err)
		nbdkit.Process.Kill()
		return nil, err
	}

	source := &VDDKDataSource{
		Command:   nbdkit,
		NbdSocket: socket,
		NbdHandle: handle,
	}
	return source, nil
}

func createVddkDataSink(destinationFile string, size uint64) (VDDKDataSink, error) {
	file, err := os.OpenFile(destinationFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	err = unix.Fadvise(int(file.Fd()), 0, int64(size), unix.MADV_SEQUENTIAL)
	if err != nil {
		klog.Warningf("Error with sequential fadvise: %v", err)
	}
	writer := bufio.NewWriter(file)
	sink := &VDDKFileSink{
		file:   file,
		writer: writer,
	}
	return sink, err
}

// Info is called to get initial information about the data.
func (vs *VDDKDataSource) Info() (ProcessingPhase, error) {
	size, err := vs.NbdHandle.GetSize()
	if err != nil {
		klog.Errorf("Unable to get size from libnbd handle: %v", err)
		return ProcessingPhaseError, err
	}
	klog.Infof("Transferring %d-byte disk image...", size)
	return ProcessingPhaseTransferDataFile, nil
}

// Close closes any readers or other open resources.
func (vs *VDDKDataSource) Close() error {
	vs.NbdHandle.Close()
	return vs.Command.Process.Kill()
}

// GetURL returns the url that the data processor can use when converting the data.
func (vs *VDDKDataSource) GetURL() *url.URL {
	return vs.NbdSocket
}

// Transfer is called to transfer the data from the source to the path passed in.
func (vs *VDDKDataSource) Transfer(path string) (ProcessingPhase, error) {
	return ProcessingPhaseTransferDataFile, nil
}

// TransferFile is called to transfer the data from the source to the file passed in.
func (vs *VDDKDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	size, err := vs.NbdHandle.GetSize()
	if err != nil {
		klog.Errorf("Unable to get size from libnbd handle: %v", err)
		return ProcessingPhaseError, err
	}

	sink, err := newVddkDataSink(destinationFile, size)
	if err != nil {
		return ProcessingPhaseError, err
	}
	defer sink.Close()

	start := uint64(0)
	lastProgressPercent := uint(0)
	lastProgressBytes := uint64(0)
	lastProgressTime := time.Now()
	initialProgressTime := time.Now()
	blocksize := uint64(1024 * 1024)
	buf := make([]byte, blocksize)
	for i := start; i < size; i += blocksize {
		if (size - i) < blocksize {
			blocksize = size - i
			buf = make([]byte, blocksize)
		}

		err = vs.NbdHandle.Pread(buf, i, nil)
		if err != nil {
			klog.Errorf("Failed to read from data source at offset %d! First error was: %v", i, err)
			retryErr := vs.NbdHandle.Pread(buf, i, nil)
			if retryErr != nil {
				klog.Errorf("Retry error was: %v", retryErr)
				return ProcessingPhaseError, err
			}
			klog.Infof("Retry was successful.")
		}

		written, err := sink.Write(buf)
		if err != nil {
			klog.Errorf("Failed to write source data to destination: %v", err)
			return ProcessingPhaseError, err
		}
		if uint64(written) < blocksize {
			klog.Errorf("Failed to write whole buffer to destination! Wrote %d/%d bytes.", written, blocksize)
			return ProcessingPhaseError, errors.New("failed to write whole buffer to destination")
		}

		// Only log progress at approximately 1% intervals.
		currentProgressBytes := i + uint64(written)
		currentProgressPercent := uint(100.0 * (float64(currentProgressBytes) / float64(size)))
		if currentProgressPercent > lastProgressPercent {
			progressMessage := fmt.Sprintf("Transferred %d/%d bytes (%d%%)", currentProgressBytes, size, currentProgressPercent)

			currentProgressTime := time.Now()
			overallProgressTime := uint64(time.Since(initialProgressTime).Seconds())
			if overallProgressTime > 0 {
				overallProgressRate := currentProgressBytes / overallProgressTime
				progressMessage += fmt.Sprintf(" at %d bytes/second overall", overallProgressRate)
			}

			progressTimeDifference := uint64(currentProgressTime.Sub(lastProgressTime).Seconds())
			if progressTimeDifference > 0 {
				progressSize := currentProgressBytes - lastProgressBytes
				progressRate := progressSize / progressTimeDifference
				progressMessage += fmt.Sprintf(", last 1%% was %d bytes at %d bytes/second", progressSize, progressRate)
			}

			klog.Info(progressMessage)

			lastProgressBytes = currentProgressBytes
			lastProgressTime = currentProgressTime
			lastProgressPercent = currentProgressPercent
		}
		v := float64(currentProgressPercent)
		metric := &dto.Metric{}
		err = progress.WithLabelValues(ownerUID).Write(metric)
		if err == nil && v > 0 && v > *metric.Counter.Value {
			progress.WithLabelValues(ownerUID).Add(v - *metric.Counter.Value)
		}
	}

	return ProcessingPhaseComplete, nil
}
