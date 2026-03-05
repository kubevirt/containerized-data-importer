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

// Package checksum provides checksum validation utilities for CDI.
// This package is intentionally kept separate from pkg/importer to avoid
// CGO dependencies (libnbd) being pulled into components that don't need them
// (e.g., cdi-operator, webhooks).
package checksum

import (
	"encoding/hex"
	"strings"

	"github.com/pkg/errors"
)

const (
	// AlgorithmMD5 represents MD5 hash algorithm
	AlgorithmMD5 = "md5"
	// AlgorithmSHA1 represents SHA-1 hash algorithm
	AlgorithmSHA1 = "sha1"
	// AlgorithmSHA256 represents SHA-256 hash algorithm
	AlgorithmSHA256 = "sha256"
	// AlgorithmSHA512 represents SHA-512 hash algorithm
	AlgorithmSHA512 = "sha512"
)

// ParseAndValidate parses and validates a checksum string
// Returns algorithm and hash value if valid
func ParseAndValidate(checksumStr string) (algorithm, hash string, err error) {
	// Parse format
	parts := strings.SplitN(checksumStr, ":", 2)
	if len(parts) != 2 {
		return "", "", errors.Errorf(
			"invalid checksum format: expected 'algorithm:hash', got '%s'",
			checksumStr,
		)
	}

	algorithm = strings.ToLower(strings.TrimSpace(parts[0]))
	hash = strings.ToLower(strings.TrimSpace(parts[1]))

	if hash == "" {
		return "", "", errors.Errorf("checksum hash value cannot be empty")
	}

	// Validate algorithm and get expected hash length
	var expectedLen int
	switch algorithm {
	case AlgorithmMD5:
		expectedLen = 32 // MD5 produces 128 bits = 32 hex chars
	case AlgorithmSHA1:
		expectedLen = 40 // SHA-1 produces 160 bits = 40 hex chars
	case AlgorithmSHA256:
		expectedLen = 64 // SHA-256 produces 256 bits = 64 hex chars
	case AlgorithmSHA512:
		expectedLen = 128 // SHA-512 produces 512 bits = 128 hex chars
	default:
		return "", "", errors.Errorf(
			"unsupported hash algorithm '%s': supported algorithms are: md5, sha1, sha256, sha512",
			algorithm,
		)
	}

	// Validate hash length
	if len(hash) != expectedLen {
		return "", "", errors.Errorf(
			"invalid %s hash length: expected %d hex characters, got %d",
			algorithm,
			expectedLen,
			len(hash),
		)
	}

	// Validate it's valid hex
	if _, err := hex.DecodeString(hash); err != nil {
		return "", "", errors.Errorf("checksum hash is not valid hexadecimal: %v", err)
	}

	return algorithm, hash, nil
}
