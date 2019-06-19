package appregistry

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/golang/mock/gomock"
	appr_models "github.com/operator-framework/go-appr/models"
)

func TestRetrieveOne_PackageExists_SuccessExpected(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	adapter := NewMockapprApiAdapter(controller)
	decoder := NewMockblobDecoder(controller)

	client := client{
		adapter: adapter,
		decoder: decoder,
	}

	namespace := "redhat"
	repository := "foo"
	release := "1.0"
	digest := "abcdefgh"

	pkg := &appr_models.Package{Content: &appr_models.OciDescriptor{
		Digest: digest,
	}}
	adapter.EXPECT().GetPackageMetadata(namespace, repository, release).Return(pkg, nil).Times(1)

	blobExpected := []byte{'e', 'n', 'c', 'o', 'd', 'e', 'd'}
	adapter.EXPECT().DownloadOperatorManifest(namespace, repository, digest).Return(blobExpected, nil).Times(1)

	decodedExpected := []byte{'d', 'e', 'c', 'o', 'd', 'e', 'd'}
	decoder.EXPECT().Decode(blobExpected).Return(decodedExpected, nil).Times(1)

	metadata, err := client.RetrieveOne(fmt.Sprintf("%s/%s", namespace, repository), release)

	assert.NoError(t, err)
	assert.Equal(t, namespace, metadata.RegistryMetadata.Namespace)
	assert.Equal(t, repository, metadata.RegistryMetadata.Repository)
	assert.Equal(t, release, metadata.RegistryMetadata.Release)
	assert.Equal(t, digest, metadata.RegistryMetadata.Digest)
	assert.Equal(t, decodedExpected, metadata.RawYAML)
}
