package ovirtclient

import (
	"io"
)

// readSeekCloser is an interface that has been backported from Go 1.16 so this library can work on Go 1.14 too.
// It can be replaced once all users have moved to Go 1.16.
type readSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}
