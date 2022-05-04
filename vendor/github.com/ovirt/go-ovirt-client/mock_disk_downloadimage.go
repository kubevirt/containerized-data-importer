package ovirtclient

import (
	"bytes"
	"io"
	"sync"
	"time"
)

// Deprecated: use StartDownloadDisk instead.
func (m *mockClient) StartImageDownload(diskID string, format ImageFormat, retries ...RetryStrategy) (
	ImageDownload,
	error,
) {
	return m.StartDownloadDisk(diskID, format, retries...)
}

func (m *mockClient) StartDownloadDisk(diskID string, format ImageFormat, _ ...RetryStrategy) (ImageDownload, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	disk, ok := m.disks[diskID]
	if !ok {
		return nil, newError(ENotFound, "disk with ID %s not found", diskID)
	}

	if disk.format != format {
		m.logger.Warningf("the image upload client requested a conversion from from %s to %s; the mock library does not support this and the source image data will be used unmodified which may lead to errors", disk.format, format)
	}

	dl := &mockImageDownload{
		disk:      disk,
		size:      0,
		bytesRead: 0,
		done:      make(chan struct{}),
		lastError: nil,
		lock:      &sync.Mutex{},
		reader:    bytes.NewReader(disk.data),
	}
	go dl.prepare()

	return dl, nil
}

// Deprecated: use DownloadDisk instead.
func (m *mockClient) DownloadImage(diskID string, format ImageFormat, retries ...RetryStrategy) (
	ImageDownloadReader,
	error,
) {
	return m.DownloadDisk(diskID, format, retries...)
}

func (m *mockClient) DownloadDisk(diskID string, format ImageFormat, retries ...RetryStrategy) (
	ImageDownloadReader,
	error,
) {
	download, err := m.StartDownloadDisk(diskID, format, retries...)
	if err != nil {
		return nil, err
	}
	<-download.Initialized()
	if err := download.Err(); err != nil {
		return nil, err
	}
	return download, nil
}

type mockImageDownload struct {
	disk      *diskWithData
	size      uint64
	bytesRead uint64
	done      chan struct{}
	lastError error
	lock      *sync.Mutex
	reader    io.Reader
}

func (m *mockImageDownload) Err() error {
	return m.lastError
}

func (m *mockImageDownload) Initialized() <-chan struct{} {
	return m.done
}

func (m *mockImageDownload) Read(p []byte) (n int, err error) {
	<-m.done
	if m.lastError != nil {
		return 0, m.lastError
	}

	n, err = m.reader.Read(p)

	m.lock.Lock()
	defer m.lock.Unlock()
	if err != nil {
		m.lastError = err
	}
	m.bytesRead += uint64(n)

	if m.bytesRead == m.size {
		go func() {
			_ = m.Close()
		}()
	}

	return n, err
}

func (m *mockImageDownload) Close() error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.disk.Unlock()
	return nil
}

func (m *mockImageDownload) BytesRead() uint64 {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.bytesRead
}

func (m *mockImageDownload) Size() uint64 {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.size
}

func (m *mockImageDownload) prepare() {
	// Sleep one second to trigger possible race condition with determining size.
	time.Sleep(time.Second)
	m.lock.Lock()
	defer m.lock.Unlock()
	m.size = uint64(len(m.disk.data))
	close(m.done)
}
