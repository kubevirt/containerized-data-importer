package appregistry

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeHappyPath(t *testing.T) {
	expected := "message in a bottle"

	// the tar ball content is base64 encoded
	encoded := "H4sIAMu2mlsAA+3RQQrCMBCF4RxlTiCJk8bzjBC0tKli4v2tdKMLqZsiwv9tHsPM4sGUXKud8u46Wj+5bfhZSvGZ4dD511zs1QXVmDpVDer8PGh04jfq8+Zem91EnA1Wz7l8vFvb/6my/F/6SUyOl9bG/OtKAAAAAAAAAAAAAAAAAIAvPAB81gebACgAAA=="
	content, err := base64Decode([]byte(encoded))
	require.NoError(t, err)

	var d blobDecoder = &blobDecoderImpl{}
	decoded, err := d.Decode([]byte(content))

	assert.Nil(t, err)
	assert.Equal(t, string(expected), string(decoded))
}

func base64Decode(encoded []byte) ([]byte, error) {
	maxlength := base64.StdEncoding.DecodedLen(len(encoded))
	decoded := make([]byte, maxlength)

	n, err := base64.StdEncoding.Decode(decoded, encoded)
	if err != nil {
		return nil, err
	}

	return decoded[:n], nil
}
