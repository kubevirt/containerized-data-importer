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
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/oci/archive"
	"github.com/containers/image/v5/pkg/blobinfocache"
	"github.com/containers/image/v5/types"
	"github.com/pkg/errors"

	"k8s.io/klog/v2"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

const (
	whFilePrefix = ".wh."
)

var errReadingLayer = errors.New("Error reading layer")

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
		return nil, errors.Wrap(err, "Could not parse image")
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
		return nil, errors.Errorf(`Invalid image name "%s", expected colon-separated transport:reference`, img)
	}
	switch parts[0] {
	case cdiv1.RegistrySchemeDocker:
		return docker.ParseReference(parts[1])
	case cdiv1.RegistrySchemeOci:
		return archive.ParseReference(parts[1])
	}
	return nil, errors.Errorf(`Invalid image name "%s", unknown transport`, img)
}

func closeImage(src types.ImageSource) {
	if err := src.Close(); err != nil {
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
	stopAtFirst,
	preallocation bool) (bool, error) {
	var reader io.ReadCloser
	reader, _, err := src.GetBlob(ctx, layer, cache)
	if err != nil {
		klog.Errorf("%v: %v", errReadingLayer, err)
		return false, fmt.Errorf("%w: %v", errReadingLayer, err)
	}
	fr, err := NewFormatReaders(reader, 0)
	if err != nil {
		klog.Errorf("%v: %v", errReadingLayer, err)
		return false, fmt.Errorf("%w: %v", errReadingLayer, err)
	}
	defer fr.Close()

	tarReader := tar.NewReader(fr.TopReader())
	found := false
	for {
		hdr, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break // End of archive
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
				return false, errors.Wrap(err, "Error sanitizing archive path")
			}

			if err = os.MkdirAll(filepath.Dir(destFile), os.ModePerm); err != nil {
				klog.Errorf("Error creating output file's directory: %v", err)
				return false, errors.Wrap(err, "Error creating output file's directory")
			}

			if _, _, err := StreamDataToFile(tarReader, destFile, preallocation); err != nil {
				klog.Errorf("Error copying file: %v", err)
				return false, errors.Wrap(err, "Error copying file")
			}

			found = true
			if stopAtFirst {
				return found, nil
			}
		}
	}

	return found, nil
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

func copyRegistryImage(url, destDir, pathPrefix, accessKey, secKey, imageArchitecture, certDir string, insecureRegistry, stopAtFirst, preallocation bool) (*types.ImageInspectInfo, error) {
	klog.Infof("Downloading image from '%v', copying file from '%v' to '%v'", url, pathPrefix, destDir)

	ctx, cancel := commandTimeoutContext()
	defer cancel()
	srcCtx := buildSourceContext(accessKey, secKey, imageArchitecture, certDir, insecureRegistry)

	src, err := readImageSource(ctx, srcCtx, url)
	if err != nil {
		return nil, err
	}
	defer closeImage(src)

	imgCloser, err := image.FromSource(ctx, srcCtx, src)
	if err != nil {
		klog.Errorf("Error retrieving image: %v", err)
		return nil, errors.Wrap(err, "Error retrieving image")
	}
	defer imgCloser.Close()

	// in the event that target is not a manifest list / image index
	if srcCtx.ArchitectureChoice != "" {
		if err := validateImagePlatformMatch(srcCtx, imgCloser); err != nil {
			klog.Errorf("Error validating architecture: %v", err)
			return nil, fmt.Errorf("Error validating architecture: %w", err)
		}
	}

	cache := blobinfocache.DefaultCache(srcCtx)
	found := false
	layers := imgCloser.LayerInfos()

	for _, layer := range layers {
		klog.Infof("Processing layer %+v", layer)

		found, err = processLayer(ctx, src, layer, destDir, pathPrefix, cache, stopAtFirst, preallocation)
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

	info, err := imgCloser.Inspect(ctx)
	if err != nil {
		return nil, err
	}

	return info, nil
}

func validateImagePlatformMatch(sys *types.SystemContext, img types.Image) error {
	config, err := img.OCIConfig(context.Background())
	if err != nil {
		return err
	}
	if config.Architecture != sys.ArchitectureChoice {
		return fmt.Errorf(`manifest image architecture: "%s" doesn't match requested architecture: "%s"`, config.Architecture, sys.ArchitectureChoice)
	}
	return nil
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
	return copyRegistryImage(url, destDir, pathPrefix, accessKey, secKey, imageArchitecture, certDir, insecureRegistry, true, preallocation)
}

// CopyRegistryImageAll download image from registry with docker image API. It will extract all files under the pathPrefix
// url: source registry url.
// destDir: the scratch space destination.
// pathPrefix: path to extract files from.
// accessKey: accessKey for the registry described in url.
// secKey: secretKey for the registry described in url.
// certDir: directory public CA keys are stored for registry identity verification
// insecureRegistry: boolean if true will allow insecure registries.
func CopyRegistryImageAll(url, destDir, pathPrefix, accessKey, secKey, certDir string, insecureRegistry, preallocation bool) (*types.ImageInspectInfo, error) {
	return copyRegistryImage(url, destDir, pathPrefix, accessKey, secKey, "", certDir, insecureRegistry, false, preallocation)
}
