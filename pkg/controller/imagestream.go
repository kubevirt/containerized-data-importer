package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/containers/image/v5/docker/reference"
	digest "github.com/opencontainers/go-digest"
	imagev1 "github.com/openshift/api/image/v1"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func getImageStream(ctx context.Context, client client.Client, imageStreamName, imageStreamNamespace string) (*imagev1.ImageStream, string, error) {
	if imageStreamName == "" || imageStreamNamespace == "" {
		return nil, "", fmt.Errorf("Missing ImageStream name or namespace")
	}

	imageStream := &imagev1.ImageStream{}
	name, tag, err := splitImageStreamName(imageStreamName)
	if err != nil {
		return nil, "", err
	}

	imageStreamNamespacedName := types.NamespacedName{
		Namespace: imageStreamNamespace,
		Name:      name,
	}

	if err := client.Get(ctx, imageStreamNamespacedName, imageStream); err != nil {
		return nil, "", err
	}

	return imageStream, tag, nil
}

func splitImageStreamName(imageStreamName string) (string, string, error) {
	if subs := strings.Split(imageStreamName, ":"); len(subs) == 1 {
		return imageStreamName, "", nil
	} else if len(subs) == 2 && len(subs[0]) > 0 && len(subs[1]) > 0 {
		return subs[0], subs[1], nil
	}
	return "", "", fmt.Errorf("Illegal ImageStream name %s", imageStreamName)
}

func getImageStreamDigest(imageStream *imagev1.ImageStream, imageStreamTag string) (string, string, error) {
	if imageStream == nil {
		return "", "", fmt.Errorf("No ImageStream")
	}

	tags := imageStream.Status.Tags
	if len(tags) == 0 {
		return "", "", fmt.Errorf("ImageStream %s has no tags", imageStream.Name)
	}

	tagIdx := 0
	if imageStreamTag != "" {
		tagIdx = slices.IndexFunc(tags, func(t imagev1.NamedTagEventList) bool { return t.Tag == imageStreamTag })
	}

	if tagIdx == -1 {
		return "", "", fmt.Errorf("ImageStream %s has no tag %s", imageStream.Name, imageStreamTag)
	}

	tag := tags[tagIdx]
	if len(tag.Items) == 0 {
		return "", "", fmt.Errorf("ImageStream %s tag %s has no items", imageStream.Name, imageStreamTag)
	}

	// Items[0] is the most recent image
	latest := tag.Items[0]
	return latest.Image, resolveImageStreamDockerRef(imageStream, tag.Tag, latest), nil
}

func resolveImageStreamDockerRef(imageStream *imagev1.ImageStream, tagName string, latest imagev1.TagEvent) string {
	specIdx := slices.IndexFunc(imageStream.Spec.Tags, func(t imagev1.TagReference) bool { return t.Name == tagName })
	if specIdx == -1 || imageStream.Spec.Tags[specIdx].ReferencePolicy.Type != imagev1.LocalTagReferencePolicy {
		return latest.DockerImageReference
	}

	repo, err := reference.ParseNormalizedNamed(imageStream.Status.DockerImageRepository)
	if err != nil {
		return latest.DockerImageReference
	}

	dgst, err := digest.Parse(latest.Image)
	if err != nil {
		return latest.DockerImageReference
	}

	local, err := reference.WithDigest(reference.TrimNamed(repo), dgst)
	if err != nil {
		return latest.DockerImageReference
	}

	return local.String()
}
