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
	"crypto/md5"  //nolint:gosec // MD5 is user-selected, not a security decision
	"crypto/sha1" //nolint:gosec // SHA1 is user-selected, not a security decision
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"

	"k8s.io/klog/v2"

	"kubevirt.io/containerized-data-importer/pkg/util/checksum"
)

// ErrChecksumMismatch is returned when the calculated checksum does not match the expected checksum.
// Use errors.Is(err, ErrChecksumMismatch) to check for this error type.
var ErrChecksumMismatch = errors.New("checksum mismatch")

// ChecksumValidator handles checksum validation for imported data
type ChecksumValidator struct {
	algorithm        string
	expectedChecksum string
	hasher           hash.Hash
}

// NewChecksumValidator creates a new checksum validator from a checksum string
// Format: "algorithm:hash", e.g., "sha256:abc123..." or "md5:def456..."
func NewChecksumValidator(checksumStr string) (*ChecksumValidator, error) {
	if checksumStr == "" {
		return nil, nil
	}

	algorithm, expectedHash, err := checksum.ParseAndValidate(checksumStr)
	if err != nil {
		return nil, err
	}

	hasher, err := createHasher(algorithm)
	if err != nil {
		return nil, err
	}

	return &ChecksumValidator{
		algorithm:        algorithm,
		expectedChecksum: expectedHash,
		hasher:           hasher,
	}, nil
}

// GetReader returns an io.Reader that calculates the checksum as data is read
func (cv *ChecksumValidator) GetReader(r io.Reader) io.Reader {
	if cv.hasher == nil {
		return r
	}
	return io.TeeReader(r, cv.hasher)
}

// Validate checks if the calculated checksum matches the expected checksum
func (cv *ChecksumValidator) Validate() error {
	calculatedChecksum := hex.EncodeToString(cv.hasher.Sum(nil))
	if calculatedChecksum != cv.expectedChecksum {
		return fmt.Errorf(
			"%w: expected %s:%s, calculated %s:%s",
			ErrChecksumMismatch,
			cv.algorithm,
			cv.expectedChecksum,
			cv.algorithm,
			calculatedChecksum,
		)
	}

	klog.Infof("Checksum verification passed: %s:%s", cv.algorithm, calculatedChecksum)
	return nil
}

// Algorithm returns the hash algorithm being used
func (cv *ChecksumValidator) Algorithm() string {
	return cv.algorithm
}

// createHasher creates a hash.Hash instance for the given algorithm
func createHasher(algorithm string) (hash.Hash, error) {
	switch algorithm {
	case checksum.AlgorithmMD5:
		return md5.New(), nil //nolint:gosec // MD5 is user-selected, not a security decision
	case checksum.AlgorithmSHA1:
		return sha1.New(), nil //nolint:gosec // SHA1 is user-selected, not a security decision
	case checksum.AlgorithmSHA256:
		return sha256.New(), nil
	case checksum.AlgorithmSHA512:
		return sha512.New(), nil
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}
}
