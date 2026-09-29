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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/types"

	"k8s.io/klog/v2"
)

const (
	whFilePrefix = ".wh."
)

var errReadingLayer = errors.New("Error reading layer")

type blobOpener func(ctx context.Context, layer types.BlobInfo) (io.ReadCloser, error)

type fileExtractor struct {
	destDir       string
	pathPrefix    string
	preallocation bool
}

func (e fileExtractor) extract(ctx context.Context, layers []types.BlobInfo, open blobOpener) error {
	for _, layer := range layers {
		klog.Infof("Processing layer %+v", layer)

		found, err := e.fromLayer(ctx, layer, open)
		if found {
			return nil
		}
		// An unreadable layer is not fatal, the file may still live in the next one.
		if err != nil && !errors.Is(err, errReadingLayer) {
			return err
		}
	}

	err := errors.New("Failed to find VM disk image file in the container image")
	klog.Error(err)
	return err
}

func (e fileExtractor) fromLayer(ctx context.Context, layer types.BlobInfo, open blobOpener) (bool, error) {
	blob, err := open(ctx, layer)
	if err != nil {
		klog.Errorf("%v: %v", errReadingLayer, err)
		return false, fmt.Errorf("%w: %v", errReadingLayer, err)
	}

	readers, err := NewFormatReaders(blob, 0, nil)
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
		if !e.wants(hdr) {
			continue
		}

		klog.Infof("File '%v' found in the layer", hdr.Name)
		if err := e.writeFile(tarReader, hdr.Name); err != nil {
			return false, err
		}
		return true, nil
	}
}

func (e fileExtractor) wants(hdr *tar.Header) bool {
	return hasPrefix(hdr.Name, e.pathPrefix) && !isWhiteout(hdr.Name) && !isDir(hdr)
}

func (e fileExtractor) writeFile(r io.Reader, name string) error {
	destFile, err := safeJoinPaths(e.destDir, name)
	if err != nil {
		klog.Errorf("Error sanitizing archive path: %v", err)
		return fmt.Errorf("Error sanitizing archive path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(destFile), os.ModePerm); err != nil {
		klog.Errorf("Error creating output file's directory: %v", err)
		return fmt.Errorf("Error creating output file's directory: %w", err)
	}

	if _, _, err := StreamDataToFile(r, destFile, e.preallocation); err != nil {
		klog.Errorf("Error copying file: %v", err)
		return fmt.Errorf("Error copying file: %w", err)
	}
	return nil
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

// Sanitize archive file pathing from "G305: Zip Slip vulnerability"
// https://security.snyk.io/research/zip-slip-vulnerability
func safeJoinPaths(dir, path string) (string, error) {
	joined := filepath.Join(dir, path)
	if strings.HasPrefix(joined, filepath.Clean(dir)+string(os.PathSeparator)) {
		return joined, nil
	}

	return "", fmt.Errorf("content filepath is tainted: %s", path)
}
