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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/types"

	"k8s.io/klog/v2"
)

var (
	errReadingLayer = errors.New("Error reading layer")

	errDiskImageNotFound = errors.New("Failed to find VM disk image file in the container image")
)

type blobOpener func(ctx context.Context, layer types.BlobInfo) (io.ReadCloser, error)

// layerDisk is a kind of registry image, holding its disk image in one of its layers.
type layerDisk interface {
	// resolve returns the candidate layers, in order, and the inspect info if the image has any.
	resolve(ctx context.Context, sys *types.SystemContext, src types.ImageSource, cache types.BlobInfoCache) ([]types.BlobInfo, *types.ImageInspectInfo, error)
	// read reports whether the layer held the disk image.
	read(ctx context.Context, layer types.BlobInfo, open blobOpener) (bool, error)
}

func extractDisk(ctx context.Context, disk layerDisk, candidates []types.BlobInfo, open blobOpener) error {
	var readErr error
	for _, layer := range candidates {
		klog.Infof("Processing layer %+v", layer)

		found, err := disk.read(ctx, layer, open)
		switch {
		case found:
			return nil
		case errors.Is(err, errReadingLayer):
			// the disk image may be in a later layer
			readErr = err
		case err != nil:
			return err
		}
	}

	if readErr != nil {
		return fmt.Errorf("%w: %w", errDiskImageNotFound, readErr)
	}
	return errDiskImageNotFound
}

func decompressLayer(ctx context.Context, layer types.BlobInfo, open blobOpener) (*FormatReaders, error) {
	blob, err := open(ctx, layer)
	if err != nil {
		return nil, err
	}
	return NewFormatReaders(blob, 0, nil)
}

type diskWriter struct {
	destDir       string
	preallocation bool
}

func (w diskWriter) write(r io.Reader, name string) error {
	destFile, err := safeJoinPaths(w.destDir, name)
	if err != nil {
		klog.Errorf("Error sanitizing archive path: %v", err)
		return fmt.Errorf("Error sanitizing archive path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(destFile), os.ModePerm); err != nil {
		klog.Errorf("Error creating output file's directory: %v", err)
		return fmt.Errorf("Error creating output file's directory: %w", err)
	}

	if _, _, err := StreamDataToFile(r, destFile, w.preallocation); err != nil {
		klog.Errorf("Error copying file: %v", err)
		return fmt.Errorf("Error copying file: %w", err)
	}
	return nil
}

// Sanitize archive file pathing from "G305: Zip Slip vulnerability"
// https://security.snyk.io/research/zip-slip-vulnerability
func safeJoinPaths(dir, path string) (string, error) {
	joined := filepath.Join(dir, path)
	if strings.HasPrefix(joined, filepath.Clean(dir)+string(os.PathSeparator)) {
		return joined, nil
	}

	return "", fmt.Errorf("content filepath is tainted: %s", path)
}
