/*
Copyright 2018 The CDI Authors.

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
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/oci/archive"
	"github.com/containers/image/v5/pkg/blobinfocache"
	"github.com/containers/image/v5/types"

	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	"kubevirt.io/containerized-data-importer/pkg/common"
)

const (
	// containerDiskImageDir - Expected disk image location in container image as described in
	// https://github.com/kubevirt/kubevirt/blob/main/docs/container-register-disks.md
	containerDiskImageDir = "disk"
)

// RegistryDataSource is the struct containing the information needed to import from a registry data source.
// Sequence of phases:
// 1. Info -> Transfer
// 2. Transfer -> Convert
type RegistryDataSource struct {
	endpoint          string
	accessKey         string
	secKey            string
	imageArchitecture string
	certDir           string
	insecureTLS       bool
	imageDir          string
	//The discovered image file in scratch space.
	url *url.URL
	//The discovered image info from the registry.
	info *types.ImageInspectInfo
}

// NewRegistryDataSource creates a new instance of the Registry Data Source.
func NewRegistryDataSource(endpoint, accessKey, secKey, imageArchitecture, certDir string, insecureTLS bool) *RegistryDataSource {
	allCertDir, err := CreateCertificateDir(certDir)
	if err != nil {
		klog.Infof("Error creating allCertDir %v", err)
		if allCertDir != "/" {
			err = os.RemoveAll(allCertDir)
			if err != nil {
				klog.Errorf("Unable to clean up all cert dir %v", err)
			}
		}
		allCertDir = certDir
	}
	return &RegistryDataSource{
		endpoint:          endpoint,
		accessKey:         accessKey,
		secKey:            secKey,
		imageArchitecture: imageArchitecture,
		certDir:           allCertDir,
		insecureTLS:       insecureTLS,
	}
}

// Info is called to get initial information about the data. No information available for registry currently.
func (rd *RegistryDataSource) Info() (ProcessingPhase, error) {
	return ProcessingPhaseTransferScratch, nil
}

// Transfer is called to transfer the data from the source registry to a temporary location.
func (rd *RegistryDataSource) Transfer(path string, preallocation bool) (ProcessingPhase, error) {
	rd.imageDir = filepath.Join(path, containerDiskImageDir)
	if err := CleanAll(rd.imageDir); err != nil {
		return ProcessingPhaseError, err
	}

	size, err := GetAvailableSpace(path)
	if err != nil {
		return ProcessingPhaseError, err
	}
	if size <= int64(0) {
		//Path provided is invalid.
		return ProcessingPhaseError, ErrInvalidPath
	}

	klog.V(1).Infof("Copying registry image to scratch space.")
	rd.info, err = CopyRegistryImage(rd.endpoint, path, containerDiskImageDir, rd.accessKey, rd.secKey, rd.imageArchitecture, rd.certDir, rd.insecureTLS, preallocation)
	if err != nil {
		return ProcessingPhaseError, fmt.Errorf("Failed to read registry image: %w", err)
	}

	imageFile, err := getImageFileName(rd.imageDir)
	if err != nil {
		return ProcessingPhaseError, fmt.Errorf("Cannot locate image file: %w", err)
	}

	// imageFile and rd.imageDir are both valid, thus the Join will be valid, and the parse will work, no need to check for parse errors
	rd.url, _ = url.Parse(filepath.Join(rd.imageDir, imageFile))
	klog.V(3).Infof("Successfully found file. VM disk image filename is %s", rd.url.String())
	return ProcessingPhaseConvert, nil
}

// TransferFile is called to transfer the data from the source to the passed in file.
func (rd *RegistryDataSource) TransferFile(fileName string, preallocation bool) (ProcessingPhase, error) {
	return ProcessingPhaseError, errors.New("Transferfile should not be called")
}

// GetURL returns the url that the data processor can use when converting the data.
func (rd *RegistryDataSource) GetURL() *url.URL {
	return rd.url
}

// GetTerminationMessage returns data to be serialized and used as the termination message of the importer.
func (rd *RegistryDataSource) GetTerminationMessage() *common.TerminationMessage {
	if rd.info == nil {
		return nil
	}
	return &common.TerminationMessage{
		Labels: envsToLabels(rd.info.Env),
	}
}

// Close closes any readers or other open resources.
func (rd *RegistryDataSource) Close() error {
	// No-op, no open readers
	return nil
}

func getImageFileName(dir string) (string, error) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		klog.Errorf("image directory does not exist")
		return "", errors.New("image directory does not exist")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		klog.Errorf("Error reading directory")
		return "", fmt.Errorf("image file does not exist in image directory: %w", err)
	}

	if len(entries) == 0 {
		klog.Errorf("image file does not exist in image directory - directory is empty ")
		return "", errors.New("image file does not exist in image directory - directory is empty")
	}

	if len(entries) > 1 {
		klog.Errorf("image directory contains more than one file")
		return "", errors.New("image directory contains more than one file")
	}

	fileinfo := entries[0]
	if fileinfo.IsDir() {
		klog.Errorf("image file does not exist in image directory contains another directory ")
		return "", errors.New("image directory contains another directory")
	}

	filename := fileinfo.Name()

	if len(strings.TrimSpace(filename)) == 0 {
		klog.Errorf("image file does not exist in image directory - file has no name ")
		return "", errors.New("image file does has no name")
	}

	klog.V(1).Infof("VM disk image filename is %s", filename)

	return filename, nil
}

// CreateCertificateDir creates a common certificate dir
func CreateCertificateDir(registryCertDir string) (string, error) {
	allCerts := "/tmp/all_certs"
	if err := os.MkdirAll(allCerts, 0700); err != nil {
		return allCerts, err
	}

	if _, err := os.Stat(common.ImporterProxyCertDir); err == nil {
		klog.Info("Copying proxy certs")
		if err := collectCerts(common.ImporterProxyCertDir, allCerts, "proxy-"); err != nil {
			return allCerts, err
		}
	}

	if registryCertDir == "" {
		klog.Info("Registry certs directory not configured")
		return allCerts, nil
	}

	klog.Info("Copying registry certs")
	if err := collectCerts(registryCertDir, allCerts, ""); err != nil {
		return allCerts, err
	}
	return allCerts, nil
}

func collectCerts(certDir, targetDir, targetPrefix string) error {
	directory, err := os.Open(certDir)
	if err != nil {
		return err
	}
	objects, err := directory.Readdir(-1)
	if err != nil {
		return err
	}
	for _, obj := range objects {
		if !strings.HasSuffix(obj.Name(), ".crt") && !strings.HasSuffix(obj.Name(), ".pem") {
			klog.Warningf("Unable to collect cert: %s Must have file extension .crt or .pem", obj.Name())
			continue
		}

		targetName := targetPrefix + obj.Name()
		if strings.HasSuffix(obj.Name(), ".pem") {
			// the containers library is currently filtering out any certs that don't have .crt file extension
			// https://github.com/containers/image/blob/df7e80d2d19872b61f352a8a182ec934dc0c2346/pkg/tlsclientconfig/tlsclientconfig.go#L36
			//
			// append .crt extension here so .pem certs can be accepted
			targetName = strings.TrimSuffix(targetName, ".pem") + ".crt"
		}

		if err := LinkFile(filepath.Join(certDir, obj.Name()), filepath.Join(targetDir, targetName)); err != nil {
			return err
		}
	}
	return nil
}

const (
	whFilePrefix = ".wh."
)

var (
	errReadingLayer = errors.New("Error reading layer")

	// ErrBootcImageDetected is returned when a bootc/ostree-bootable container image is detected
	// but conversion is not yet implemented.
	ErrBootcImageDetected = errors.New("bootc image detected: this image contains an ostree-based bootable OS (containers.bootc=1 or ostree.bootable=1) and cannot be imported as a regular container disk; bootc-to-disk conversion is not yet implemented")
)

func commandTimeoutContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func buildSourceContext(accessKey, secKey, imageArchitecture, certDir string, insecureRegistry bool) *types.SystemContext {
	ctx := &types.SystemContext{}
	if accessKey != "" && secKey != "" {
		ctx.DockerAuthConfig = &types.DockerAuthConfig{
			Username: accessKey,
			Password: secKey,
		}
	}
	if certDir != "" {
		ctx.DockerCertPath = certDir
		ctx.DockerDaemonCertPath = certDir
	}

	if insecureRegistry {
		ctx.DockerDaemonInsecureSkipTLSVerify = true
		ctx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(true)
	}

	if imageArchitecture != "" {
		ctx.ArchitectureChoice = imageArchitecture
	}

	return ctx
}

func readImageSource(ctx context.Context, sys *types.SystemContext, img string) (types.ImageSource, error) {
	ref, err := parseImageName(img)
	if err != nil {
		klog.Errorf("Could not parse image: %v", err)
		return nil, fmt.Errorf("Could not parse image: %w", err)
	}

	src, err := ref.NewImageSource(ctx, sys)
	if err != nil {
		klog.Errorf("Could not create image reference: %v", err)
		return nil, NewImagePullFailedError(err)
	}
	return src, nil
}

func parseImageName(img string) (types.ImageReference, error) {
	parts := strings.SplitN(img, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf(`Invalid image name "%s", expected colon-separated transport:reference`, img)
	}
	switch parts[0] {
	case cdiv1.RegistrySchemeDocker:
		return docker.ParseReference(parts[1])
	case cdiv1.RegistrySchemeOci:
		return archive.ParseReference(parts[1])
	}
	return nil, fmt.Errorf(`Invalid image name "%s", unknown transport`, img)
}

func closeImage(c io.Closer) {
	if err := c.Close(); err != nil {
		klog.Warningf("Could not close image source: %v ", err)
	}
}

func hasPrefix(path string, pathPrefix string) bool {
	return strings.HasPrefix(path, pathPrefix) ||
		strings.HasPrefix(path, "./"+pathPrefix)
}

func isWhiteout(path string) bool {
	return strings.HasPrefix(filepath.Base(path), whFilePrefix)
}

func isDir(hdr *tar.Header) bool {
	return hdr.Typeflag == tar.TypeDir
}

func processLayer(ctx context.Context,
	src types.ImageSource,
	layer types.BlobInfo,
	destDir string,
	pathPrefix string,
	cache types.BlobInfoCache,
	preallocation bool) (bool, error) {
	var reader io.ReadCloser
	reader, _, err := src.GetBlob(ctx, layer, cache)
	if err != nil {
		klog.Errorf("%v: %v", errReadingLayer, err)
		return false, fmt.Errorf("%w: %v", errReadingLayer, err)
	}
	fr, err := NewFormatReaders(reader, 0, nil)
	if err != nil {
		klog.Errorf("%v: %v", errReadingLayer, err)
		return false, fmt.Errorf("%w: %v", errReadingLayer, err)
	}
	defer fr.Close()

	tarReader := tar.NewReader(fr.TopReader())
	for {
		hdr, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return false, nil // End of archive
		}
		if err != nil {
			klog.Errorf("%v: %v", errReadingLayer, err)
			return false, fmt.Errorf("%w: %v", errReadingLayer, err)
		}

		if hasPrefix(hdr.Name, pathPrefix) && !isWhiteout(hdr.Name) && !isDir(hdr) {
			klog.Infof("File '%v' found in the layer", hdr.Name)
			destFile, err := safeJoinPaths(destDir, hdr.Name)
			if err != nil {
				klog.Errorf("Error sanitizing archive path: %v", err)
				return false, fmt.Errorf("Error sanitizing archive path: %w", err)
			}

			if err = os.MkdirAll(filepath.Dir(destFile), os.ModePerm); err != nil {
				klog.Errorf("Error creating output file's directory: %v", err)
				return false, fmt.Errorf("Error creating output file's directory: %w", err)
			}

			if _, _, err := StreamDataToFile(tarReader, destFile, preallocation); err != nil {
				klog.Errorf("Error copying file: %v", err)
				return false, fmt.Errorf("Error copying file: %w", err)
			}

			return true, nil
		}
	}
}

// Sanitize archive file pathing from "G305: Zip Slip vulnerability"
// https://security.snyk.io/research/zip-slip-vulnerability
func safeJoinPaths(dir, path string) (v string, err error) {
	v = filepath.Join(dir, path)
	wantPrefix := filepath.Clean(dir) + string(os.PathSeparator)

	if strings.HasPrefix(v, wantPrefix) {
		return v, nil
	}

	return "", fmt.Errorf("%s: %s", "content filepath is tainted", path)
}

func copyRegistryImage(url, destDir, pathPrefix, accessKey, secKey, imageArchitecture, certDir string, insecureRegistry, preallocation bool) (*types.ImageInspectInfo, error) {
	klog.Infof("Downloading image from '%v', copying file from '%v' to '%v'", url, pathPrefix, destDir)

	ctx, cancel := commandTimeoutContext()
	defer cancel()
	srcCtx := buildSourceContext(accessKey, secKey, imageArchitecture, certDir, insecureRegistry)

	src, err := readImageSource(ctx, srcCtx, url)
	if err != nil {
		return nil, err
	}

	imgCloser, err := image.FromSource(ctx, srcCtx, src)
	if err != nil {
		closeImage(src)
		klog.Errorf("Error retrieving image: %v", err)
		return nil, fmt.Errorf("Error retrieving image: %w", err)
	}
	defer closeImage(imgCloser)

	// The config the checks below read is also what the caller gets back, so inspect once.
	info, err := imgCloser.Inspect(ctx)
	if err != nil {
		return nil, fmt.Errorf("Error inspecting image: %w", err)
	}
	if err := validateImagePlatformMatch(srcCtx, info); err != nil {
		return nil, err
	}
	if err := checkBootcImage(info); err != nil {
		return nil, err
	}

	cache := blobinfocache.DefaultCache(srcCtx)
	found := false
	layers := imgCloser.LayerInfos()

	for _, layer := range layers {
		klog.Infof("Processing layer %+v", layer)

		found, err = processLayer(ctx, src, layer, destDir, pathPrefix, cache, preallocation)
		if found {
			break
		}
		if err != nil {
			if !errors.Is(err, errReadingLayer) {
				return nil, err
			}
			// Skipping layer and trying the next one.
			// Error already logged in processLayer
			continue
		}
	}

	if !found {
		klog.Errorf("Failed to find VM disk image file in the container image")
		return nil, errors.New("Failed to find VM disk image file in the container image")
	}

	return info, nil
}

const (
	bootcImageLabel   = "containers.bootc"
	ostreeBootLabel   = "ostree.bootable"
	bootcLabelEnabled = "1"
)

func checkBootcImage(info *types.ImageInspectInfo) error {
	if info.Labels[bootcImageLabel] == bootcLabelEnabled ||
		info.Labels[ostreeBootLabel] == bootcLabelEnabled {
		klog.Infof("Detected bootc/ostree-bootable container image")
		return ErrBootcImageDetected
	}
	return nil
}

func validateImagePlatformMatch(sys *types.SystemContext, info *types.ImageInspectInfo) error {
	if sys.ArchitectureChoice == "" || info.Architecture == sys.ArchitectureChoice {
		return nil
	}
	return fmt.Errorf(`Error validating architecture: manifest image architecture: "%s" doesn't match requested architecture: "%s"`,
		info.Architecture, sys.ArchitectureChoice)
}

// GetImageDigest returns the digest of the container image at url.
// url: source registry url.
// accessKey: accessKey for the registry described in url.
// secKey: secretKey for the registry described in url.
// certDir: directory public CA keys are stored for registry identity verification
// insecureRegistry: boolean if true will allow insecure registries.
func GetImageDigest(url, accessKey, secKey, certDir string, insecureRegistry bool) (string, error) {
	klog.Infof("Inspecting image from '%v'", url)

	ctx, cancel := commandTimeoutContext()
	defer cancel()
	srcCtx := buildSourceContext(accessKey, secKey, "", certDir, insecureRegistry)

	src, err := readImageSource(ctx, srcCtx, url)
	if err != nil {
		return "", err
	}
	defer closeImage(src)

	imageManifest, _, err := src.GetManifest(context.Background(), nil)
	if err != nil {
		return "", err
	}

	digest, err := manifest.Digest(imageManifest)
	if err != nil {
		return "", err
	}

	return digest.String(), nil
}

// CopyRegistryImage download image from registry with docker image API. It will extract first file under the pathPrefix
// url: source registry url.
// destDir: the scratch space destination.
// pathPrefix: path to extract files from.
// accessKey: accessKey for the registry described in url.
// secKey: secretKey for the registry described in url.
// imageArchitecture: image index filter for CPU architecture.
// certDir: directory public CA keys are stored for registry identity verification
// insecureRegistry: boolean if true will allow insecure registries.
func CopyRegistryImage(url, destDir, pathPrefix, accessKey, secKey, imageArchitecture, certDir string, insecureRegistry, preallocation bool) (*types.ImageInspectInfo, error) {
	return copyRegistryImage(url, destDir, pathPrefix, accessKey, secKey, imageArchitecture, certDir, insecureRegistry, preallocation)
}
