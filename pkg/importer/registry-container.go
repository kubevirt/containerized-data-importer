/*
Copyright 2026 The CDI Authors.

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
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/image"
	"github.com/containers/image/v5/types"

	"k8s.io/klog/v2"
)

// ErrBootcImageDetected is returned when a bootc/ostree-bootable container image is detected
// but conversion is not yet implemented.
var ErrBootcImageDetected = errors.New("bootc image detected: this image contains an ostree-based bootable OS (containers.bootc=1 or ostree.bootable=1) and cannot be imported as a regular container disk; bootc-to-disk conversion is not yet implemented")

const (
	whFilePrefix = ".wh."
)

// containerDisk is a container disk image, the disk image being the first file under pathPrefix.
type containerDisk struct {
	disk       diskWriter
	pathPrefix string
}

func (c containerDisk) resolve(ctx context.Context, sys *types.SystemContext, src types.ImageSource, _ types.BlobInfoCache) ([]types.BlobInfo, *types.ImageInspectInfo, error) {
	img, err := image.FromUnparsedImage(ctx, sys, image.UnparsedInstance(src, nil))
	if err != nil {
		klog.Errorf("Error retrieving image: %v", err)
		return nil, nil, fmt.Errorf("Error retrieving image: %w", err)
	}

	info, err := img.Inspect(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("Error inspecting image: %w", err)
	}
	if err := validateImagePlatformMatch(sys, info); err != nil {
		return nil, nil, err
	}
	if err := checkBootcImage(info); err != nil {
		return nil, nil, err
	}
	return img.LayerInfos(), info, nil
}

func (c containerDisk) read(ctx context.Context, layer types.BlobInfo, open blobOpener) (bool, error) {
	readers, err := decompressLayer(ctx, layer, open)
	if err != nil {
		klog.Errorf("%v: %v", errReadingLayer, err)
		return false, fmt.Errorf("%w: %v", errReadingLayer, err)
	}
	defer readers.Close()

	tarReader := tar.NewReader(readers.TopReader())
	for {
		hdr, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return false, nil // End of archive
		}
		if err != nil {
			klog.Errorf("%v: %v", errReadingLayer, err)
			return false, fmt.Errorf("%w: %v", errReadingLayer, err)
		}
		if !c.wants(hdr) {
			continue
		}

		klog.Infof("File '%v' found in the layer", hdr.Name)
		if err := c.disk.write(tarReader, hdr.Name); err != nil {
			return false, err
		}
		return true, nil
	}
}

func (c containerDisk) wants(hdr *tar.Header) bool {
	return hasPrefix(hdr.Name, c.pathPrefix) && !isWhiteout(hdr.Name) && !isDir(hdr)
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
