package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/util"
	prometheusutil "kubevirt.io/containerized-data-importer/pkg/util/prometheus"
)

var (
	contentType string
	mountPoint  string
	uploadBytes uint64
)

type execReader struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (er *execReader) Read(p []byte) (n int, err error) {
	n, err = er.stdout.Read(p)
	if err == io.EOF {
		if err2 := er.cmd.Wait(); err2 != nil {
			errBytes, _ := ioutil.ReadAll(er.stderr)
			klog.Fatalf("Subprocess did not execute successfully, result is: %q\n%s", er.cmd.ProcessState.ExitCode(), string(errBytes))
		}
	}
	return
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
		klog.Fatalf("Error geting env var %s", name)
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
	}
	tlsConfig.BuildNameToCertificate()

	transport := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{Transport: transport}

	return client
}

func startPrometheus() {
	certsDirectory, err := ioutil.TempDir("", "certsdir")
	if err != nil {
		klog.Fatalf("Error %s creating temp dir", err)
	}

	prometheusutil.StartPrometheusEndpoint(certsDirectory)
}

func createProgressReader(readCloser io.ReadCloser, ownerUID string, totalBytes uint64) io.ReadCloser {
	progress := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "clone_progress",
			Help: "The clone progress in percentage",
		},
		[]string{"ownerUID"},
	)
	prometheus.MustRegister(progress)

	promReader := prometheusutil.NewProgressReader(readCloser, totalBytes, progress, ownerUID)
	promReader.StartTimedUpdate()

	return promReader
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

func newTarReader() (io.ReadCloser, error) {
	cmd := exec.Command("/usr/bin/tar", "Scv", ".")
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

	return &execReader{cmd: cmd, stdout: stdout, stderr: ioutil.NopCloser(&stderr)}, nil
}

func getInputStream() (rc io.ReadCloser) {
	var err error
	switch contentType {
	case "filesystem-clone":
		rc, err = newTarReader()
		if err != nil {
			klog.Fatalf("Error creating tar reader for %q: %+v", mountPoint, err)
		}
	case "blockdevice-clone":
		rc, err = os.Open(mountPoint)
		if err != nil {
			klog.Fatalf("Error opening block device %q: %+v", mountPoint, err)
		}
	default:
		klog.Fatalf("Invalid content-type %q", contentType)
	}
	return
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

	klog.V(1).Infoln("Starting cloner target")

	reader := pipeToSnappy(createProgressReader(getInputStream(), ownerUID, uploadBytes))

	startPrometheus()

	client := createHTTPClient(clientKey, clientCert, serverCert)

	req, _ := http.NewRequest("POST", url, reader)

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
	err = util.WriteTerminationMessage("Clone Complete")
	if err != nil {
		klog.Errorf("%+v", err)
		os.Exit(1)
	}
}
