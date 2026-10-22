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
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"

	cdiv1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
)

// artifactDiskImageName names the disk image, since a layer blob carries no file name.
const artifactDiskImageName = "disk.img"

var (
	errEmptyLayerSelector = errors.New("The layer selector has no annotations to match")
	errLayerNotFound      = errors.New("No layer of the manifest carries the selected annotations")
	errLayerAmbiguous     = errors.New("The selected annotations have to match a single layer of the manifest")
)

// artifactDisk is an OCI artifact, the disk image being the whole blob of the selected layer.
type artifactDisk struct {
	disk       diskWriter
	pathPrefix string
	selector   *cdiv1.LayerSelector
}

func (a artifactDisk) resolve(ctx context.Context, sys *types.SystemContext, src types.ImageSource, _ types.BlobInfoCache) ([]types.BlobInfo, *types.ImageInspectInfo, error) {
	layers, err := resolveArtifactLayers(ctx, sys, src)
	if err != nil {
		return nil, nil, err
	}
	layer, err := selectLayer(layers, a.selector)
	if err != nil {
		return nil, nil, err
	}
	return []types.BlobInfo{layer}, nil, nil
}

func (a artifactDisk) read(ctx context.Context, layer types.BlobInfo, open blobOpener) (bool, error) {
	readers, err := decompressLayer(ctx, layer, open)
	if err != nil {
		return false, fmt.Errorf("Error reading layer %v: %w", layer.Digest, err)
	}
	defer readers.Close()

	if err := a.disk.write(readers.TopReader(), filepath.Join(a.pathPrefix, artifactDiskImageName)); err != nil {
		return false, err
	}
	return true, nil
}

func selectLayer(layers []types.BlobInfo, selector *cdiv1.LayerSelector) (types.BlobInfo, error) {
	matchAnnotations := selector.MatchAnnotations
	if len(matchAnnotations) == 0 {
		// matching nothing would match every layer
		return types.BlobInfo{}, errEmptyLayerSelector
	}

	var matched []types.BlobInfo
	for _, layer := range layers {
		if matchesAnnotations(layer.Annotations, matchAnnotations) {
			matched = append(matched, layer)
		}
	}

	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return types.BlobInfo{}, fmt.Errorf("%w: %v", errLayerNotFound, matchAnnotations)
	default:
		digests := make([]string, 0, len(matched))
		for _, layer := range matched {
			digests = append(digests, layer.Digest.String())
		}
		return types.BlobInfo{}, fmt.Errorf("%w: %v match %d layers (%s)",
			errLayerAmbiguous, matchAnnotations, len(matched), strings.Join(digests, ", "))
	}
}

func matchesAnnotations(annotations, matchAnnotations map[string]string) bool {
	for key, value := range matchAnnotations {
		if got, ok := annotations[key]; !ok || got != value {
			return false
		}
	}
	return true
}

func resolveArtifactLayers(ctx context.Context, sys *types.SystemContext, src types.ImageSource) ([]types.BlobInfo, error) {
	man, err := resolveArtifactManifest(ctx, sys, src)
	if err != nil {
		return nil, err
	}

	layers := make([]types.BlobInfo, 0, len(man.Layers))
	for _, layer := range man.Layers {
		layers = append(layers, manifest.BlobInfoFromOCI1Descriptor(layer))
	}
	return layers, nil
}

func resolveArtifactManifest(ctx context.Context, sys *types.SystemContext, src types.ImageSource) (*manifest.OCI1, error) {
	manBlob, mimeType, err := getManifest(ctx, src, nil)
	if err != nil {
		return nil, err
	}

	if manifest.MIMETypeIsMultiImage(mimeType) {
		list, err := manifest.ListFromBlob(manBlob, mimeType)
		if err != nil {
			return nil, fmt.Errorf("Error parsing image index: %w", err)
		}
		instance, err := list.ChooseInstance(sys)
		if err != nil {
			return nil, fmt.Errorf("Error selecting a manifest from the image index: %w", err)
		}
		if manBlob, mimeType, err = getManifest(ctx, src, &instance); err != nil {
			return nil, err
		}
	}

	// a plain manifest declares no platform, the requested one only selects from an index
	return parseOCIManifest(manBlob, mimeType)
}

func getManifest(ctx context.Context, src types.ImageSource, instanceDigest *digest.Digest) ([]byte, string, error) {
	manBlob, mimeType, err := src.GetManifest(ctx, instanceDigest)
	if err != nil {
		return nil, "", fmt.Errorf("Error retrieving manifest: %w", err)
	}
	if mimeType == "" {
		mimeType = manifest.GuessMIMEType(manBlob)
	}
	return manBlob, mimeType, nil
}

func parseOCIManifest(manBlob []byte, mimeType string) (*manifest.OCI1, error) {
	if mimeType != imgspecv1.MediaTypeImageManifest {
		return nil, fmt.Errorf("Layer selection requires an OCI image manifest, got %q", mimeType)
	}
	return manifest.OCI1FromManifest(manBlob)
}
