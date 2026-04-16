package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/golang/snappy"
	"golang.org/x/sys/unix"

	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
	metrics "kubevirt.io/containerized-data-importer/pkg/monitoring/metrics/cdi-cloner"
	"kubevirt.io/containerized-data-importer/pkg/util"
	prometheusutil "kubevirt.io/containerized-data-importer/pkg/util/prometheus"
)

var (
	contentType string
	mountPoint  string
	uploadBytes uint64
)

// seekFunc is a function variable for unix.Seek, allowing test injection.
var seekFunc = func(fd int, offset int64, whence int) (int64, error) {
	return unix.Seek(fd, offset, whence)
}

type execReader struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (er *execReader) Read(p []byte) (int, error) {
	n, err := er.stdout.Read(p)
	if err == nil {
		return n, nil
	} else if !errors.Is(err, io.EOF) {
		return n, err
	}

	if err := er.cmd.Wait(); err != nil {
		errBytes, _ := io.ReadAll(er.stderr)
		klog.Fatalf("Subprocess did not execute successfully, result is: %q\n%s", er.cmd.ProcessState.ExitCode(), string(errBytes))
	}

	return n, io.EOF
}

func (er *execReader) Close() error {
	return er.stdout.Close()
}

func init() {
	flag.StringVar(&contentType, "content-type", "", "filesystem-clone|blockdevice-clone")
	flag.StringVar(&mountPoint, "mount", "", "pvc mount point")
	flag.Uint64Var(&uploadBytes, "upload-bytes", 0, "approx number of bytes in input")
	klog.InitFlags(nil)
}

func getEnvVarOrDie(name string) string {
	value := os.Getenv(name)
	if value == "" {
		klog.Fatalf("Error getting env var %s", name)
	}
	return value
}

func createHTTPClient(clientKey, clientCert, serverCert []byte) *http.Client {
	clientKeyPair, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		klog.Fatalf("Error %s creating client keypair", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(serverCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientKeyPair},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
	}

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	return client
}

func startPrometheus() {
	certsDirectory, err := os.MkdirTemp("", "certsdir")
	if err != nil {
		klog.Fatalf("Error %s creating temp dir", err)
	}

	prometheusutil.StartPrometheusEndpoint(certsDirectory)
}

func createProgressReader(readCloser io.ReadCloser, ownerUID string, totalBytes uint64) (io.ReadCloser, error) {
	if err := metrics.SetupMetrics(); err != nil {
		return nil, err
	}
	promReader := prometheusutil.NewProgressReader(readCloser, metrics.Progress(ownerUID), totalBytes)
	promReader.StartTimedUpdate()

	return promReader, nil
}

func pipeToSnappy(reader io.ReadCloser) io.ReadCloser {
	pr, pw := io.Pipe()
	sbw := snappy.NewBufferedWriter(pw)

	go func() {
		n, err := io.Copy(sbw, reader)
		if err != nil {
			klog.Fatalf("Error %s piping to gzip", err)
		}
		if err = sbw.Close(); err != nil {
			klog.Fatalf("Error closing snappy writer %+v", err)
		}
		if err = pw.Close(); err != nil {
			klog.Fatalf("Error closing pipe writer %+v", err)
		}
		klog.Infof("Wrote %d bytes\n", n)
	}()

	return pr
}

func validateContentType() {
	switch contentType {
	case "filesystem-clone", "blockdevice-clone":
	default:
		klog.Fatalf("Invalid content-type %q", contentType)
	}
}

func validateMount() {
	if mountPoint == "" {
		klog.Fatalf("Invalid mount %q", mountPoint)
	}
}

// seekHoleSupported probes whether SEEK_HOLE works on the given file.
// Filesystems without real SEEK_HOLE support (e.g. NFS < 4.2) use the
// generic VFS fallback which returns file-size, making tar -S think
// there are no holes. We replicate the same heuristic GNU tar uses:
// if SEEK_DATA returns 0 and SEEK_HOLE returns file-size on the first
// call, SEEK_HOLE is not efficiently implemented.
func seekHoleSupported(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No disk.img — likely an archive of small files where
			// raw hole-scanning overhead is negligible.
			klog.Infof("No %s found, skipping SEEK_HOLE probe", path)
			return true
		}
		klog.Fatalf("Failed to open %s for SEEK_HOLE probe: %v", path, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		klog.Fatalf("Failed to stat %s: %v", path, err)
	}
	if fi.Size() == 0 {
		return true
	}

	dataOff, dataErr := seekFunc(int(f.Fd()), 0, unix.SEEK_DATA)
	if errors.Is(dataErr, unix.ENXIO) {
		// ENXIO from SEEK_DATA means the file is entirely sparse (no data).
		// Only a real SEEK_HOLE implementation returns this.
		return true
	}

	holeOff, holeErr := seekFunc(int(f.Fd()), 0, unix.SEEK_HOLE)

	if dataErr != nil || holeErr != nil {
		klog.Warningf("SEEK_HOLE probe failed on %s: SEEK_DATA=%v, SEEK_HOLE=%v", path, dataErr, holeErr)
		return false
	}

	if dataOff == 0 && holeOff == fi.Size() {
		klog.Infof("SEEK_HOLE not supported on %s (VFS fallback detected), omitting tar -S", path)
		return false
	}

	return true
}

func newTarReader(preallocation bool) (io.ReadCloser, error) {
	excludeMap := map[string]struct{}{
		"lost+found": {},
	}

	const path = "/usr/bin/tar"
	args := []string{"cv"}
	if !preallocation && seekHoleSupported(filepath.Join(mountPoint, common.DiskImageName)) {
		// -S is used to handle sparse files. It can only be used when preallocation
		// is not requested and the filesystem supports efficient SEEK_HOLE.
		args = append(args, "-S")
	}

	files, err := os.ReadDir(mountPoint)
	if err != nil {
		return nil, err
	}

	var tarFiles []string
	for _, f := range files {
		if _, ok := excludeMap[f.Name()]; ok {
			continue
		}
		tarFiles = append(tarFiles, f.Name())
	}

	if len(tarFiles) > 0 {
		args = append(args, tarFiles...)
	} else {
		args = append(args, "--files-from", "/dev/null")
	}

	klog.Infof("Executing %s %+v", path, args)

	cmd := exec.Command(path, args...)
	cmd.Dir = mountPoint

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err = cmd.Start(); err != nil {
		return nil, err
	}

	return &execReader{cmd: cmd, stdout: stdout, stderr: io.NopCloser(&stderr)}, nil
}

func getInputStream(preallocation bool) io.ReadCloser {
	switch contentType {
	case "filesystem-clone":
		rc, err := newTarReader(preallocation)
		if err != nil {
			klog.Fatalf("Error creating tar reader for %q: %+v", mountPoint, err)
		}
		return rc
	case "blockdevice-clone":
		rc, err := os.Open(mountPoint)
		if err != nil {
			klog.Fatalf("Error opening block device %q: %+v", mountPoint, err)
		}
		return rc
	default:
		klog.Fatalf("Invalid content-type %q", contentType)
	}

	return nil
}

func main() {
	flag.Parse()
	defer klog.Flush()

	klog.Infof("content-type is %q\n", contentType)
	klog.Infof("mount is %q\n", mountPoint)
	klog.Infof("upload-bytes is %d", uploadBytes)

	validateContentType()
	validateMount()

	ownerUID := getEnvVarOrDie(common.OwnerUID)

	clientKey := []byte(getEnvVarOrDie("CLIENT_KEY"))
	clientCert := []byte(getEnvVarOrDie("CLIENT_CERT"))
	serverCert := []byte(getEnvVarOrDie("SERVER_CA_CERT"))

	url := getEnvVarOrDie("UPLOAD_URL")
	preallocation, err := strconv.ParseBool(getEnvVarOrDie(common.Preallocation)) // False is default in case of error
	if err != nil {
		klog.V(3).Infof("Preallocation variable (%s) not set, defaulting to 'false'", common.Preallocation)
	}

	klog.V(1).Infoln("Starting cloner target")

	progressReader, err := createProgressReader(getInputStream(preallocation), ownerUID, uploadBytes)
	if err != nil {
		klog.Fatalf("Error creating progress reader: %v", err)
	}
	reader := pipeToSnappy(progressReader)

	startPrometheus()

	client := createHTTPClient(clientKey, clientCert, serverCert)

	req, _ := http.NewRequest(http.MethodPost, url, reader)

	if contentType != "" {
		req.Header.Set("x-cdi-content-type", contentType)
		klog.Infof("Set header to %s", contentType)
	}

	response, err := client.Do(req)
	if err != nil {
		klog.Fatalf("Error %s POSTing to %s", err, url)
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		klog.Fatalf("Unexpected status code %d", response.StatusCode)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, response.Body)
	if err != nil {
		klog.Fatalf("Error %s copying response body", err)
	}

	klog.V(1).Infof("Response body:\n%s", buf.String())

	klog.V(1).Infoln("clone complete")
	message := "Clone Complete"
	if preallocation {
		message += ", " + common.PreallocationApplied
	}
	err = util.WriteTerminationMessage(message)
	if err != nil {
		klog.Errorf("%+v", err)
		os.Exit(1)
	}
}
