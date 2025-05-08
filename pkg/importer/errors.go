package importer

import (
	"errors"
	"fmt"
	"syscall"

	"kubevirt.io/containerized-data-importer/pkg/common"
)

// ValidationSizeError is an error indication size validation failure.
type ValidationSizeError struct {
	err error
}

func (e ValidationSizeError) Error() string { return e.err.Error() }

// ErrRequiresScratchSpace indicates that we require scratch space.
var ErrRequiresScratchSpace = fmt.Errorf(common.ScratchSpaceRequired)

// ErrInvalidPath indicates that the path is invalid.
var ErrInvalidPath = fmt.Errorf("invalid transfer path")

// ImagePullFailedError indicates that the importer failed to pull an image; This error type wraps the actual error.
type ImagePullFailedError struct {
	err error
}

// NewImagePullFailedError creates new ImagePullFailedError error object, with embedded error.
//
// Use the err parameter fot the actual wrapped error
func NewImagePullFailedError(err error) *ImagePullFailedError {
	return &ImagePullFailedError{
		err: err,
	}
}

func (err *ImagePullFailedError) Error() string {
	return fmt.Sprintf("%s: %s", common.ImagePullFailureText, err.err.Error())
}

func (err *ImagePullFailedError) Unwrap() error {
	return err.err
}

func IsNoCapacityError(err error) bool {
	return errors.Is(err, syscall.ENOSPC) ||
		errors.Is(err, syscall.EDQUOT) ||
		errors.As(err, &ValidationSizeError{})
}
