package ovirtclient

import (
	"fmt"
	"io"
	"io/ioutil"
	"sync"
)

func (m *mockClient) StartImageUpload(
	alias string,
	storageDomainID string,
	sparse bool,
	size uint64,
	reader readSeekCloser,
	_ ...RetryStrategy,
) (UploadImageProgress, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if alias == "" {
		return nil, newError(EBadArgument, "alias cannot be empty")
	}
	if _, ok := m.storageDomains[storageDomainID]; !ok {
		return nil, newError(ENotFound, "storage domain with ID %s not found", storageDomainID)
	}

	format, _, err := extractQCOWParameters(size, reader)
	if err != nil {
		return nil, err
	}

	progress := &mockImageUploadProgress{
		err: nil,
		disk: &diskWithData{
			disk: disk{
				id:              "",
				alias:           alias,
				provisionedSize: size,
				format:          format,
				storageDomainID: storageDomainID,
			},
			locked: false,
			lock:   &sync.Mutex{},
		},
		correlationID: fmt.Sprintf("image_transfer_%s", alias),
		client:        m,
		reader:        reader,
		size:          size,
		done:          make(chan struct{}),
		sparse:        sparse,
	}

	if err := progress.disk.Lock(); err != nil {
		return nil, newError(EDiskLocked, "disk locked after creation")
	}

	go progress.do()

	return progress, nil
}

func (m *mockClient) UploadImage(
	alias string,
	storageDomainID string,
	sparse bool,
	size uint64,
	reader readSeekCloser,
	retries ...RetryStrategy,
) (UploadImageResult, error) {
	progress, err := m.StartImageUpload(alias, storageDomainID, sparse, size, reader, retries...)
	if err != nil {
		return nil, err
	}
	<-progress.Done()
	if err := progress.Err(); err != nil {
		return nil, err
	}
	return progress, nil
}

type mockImageUploadProgress struct {
	err           error
	disk          *diskWithData
	correlationID string
	client        *mockClient
	reader        readSeekCloser
	size          uint64
	uploadedBytes uint64
	done          chan struct{}
	sparse        bool
}

func (m *mockImageUploadProgress) Disk() Disk {
	disk := m.disk
	if disk.id == "" {
		return nil
	}
	return disk
}

func (m *mockImageUploadProgress) CorrelationID() string {
	return m.correlationID
}

func (m *mockImageUploadProgress) UploadedBytes() uint64 {
	return m.uploadedBytes
}

func (m *mockImageUploadProgress) TotalBytes() uint64 {
	return m.size
}

func (m *mockImageUploadProgress) Err() error {
	return m.err
}

func (m *mockImageUploadProgress) Done() <-chan struct{} {
	return m.done
}

func (m *mockImageUploadProgress) do() {
	m.client.lock.Lock()
	d := m.disk
	d.id = m.client.GenerateUUID()
	m.client.disks[d.id] = d
	m.disk = d
	m.client.lock.Unlock()
	defer func() {
		m.disk.Unlock()
		close(m.done)
	}()

	var err error
	if _, err = m.reader.Seek(0, io.SeekStart); err != nil {
		m.err = fmt.Errorf("failed to seek to start of image file (%w)", err)
		return
	}
	m.disk.data, err = ioutil.ReadAll(m.reader)
	m.err = err
	if err != nil {
		m.uploadedBytes = m.size
	}
}
