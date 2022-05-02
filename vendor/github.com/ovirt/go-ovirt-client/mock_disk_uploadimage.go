package ovirtclient

import (
	"fmt"
	"io"
	"io/ioutil"
)

func (m *mockClient) StartImageUpload(
	alias string,
	storageDomainID string,
	sparse bool,
	size uint64,
	reader io.ReadSeekCloser,
	retries ...RetryStrategy,
) (UploadImageProgress, error) {
	return m.StartUploadToNewDisk(
		storageDomainID,
		"",
		size,
		CreateDiskParams().MustWithSparse(sparse).MustWithAlias(alias),
		reader,
		retries...,
	)
}

func (m *mockClient) UploadImage(
	alias string,
	storageDomainID string,
	sparse bool,
	size uint64,
	reader io.ReadSeekCloser,
	retries ...RetryStrategy,
) (UploadImageResult, error) {
	return m.UploadToNewDisk(
		storageDomainID,
		"",
		size,
		CreateDiskParams().MustWithSparse(sparse).MustWithAlias(alias),
		reader,
		retries...,
	)
}

func (m *mockClient) StartUploadToDisk(
	diskID string,
	size uint64,
	reader io.ReadSeekCloser,
	retries ...RetryStrategy,
) (UploadImageProgress, error) {
	disk, err := m.getDisk(diskID, retries...)
	if err != nil {
		return nil, err
	}

	imageFormat, qcowSize, err := extractQCOWParameters(size, reader)
	if err != nil {
		return nil, err
	}

	if qcowSize > disk.TotalSize() {
		return nil, newError(
			EBadArgument,
			"the specified size (%d bytes) is larger than the target disk %s (%d bytes)",
			size,
			diskID,
			disk.TotalSize(),
		)
	}

	if imageFormat != disk.Format() {
		return nil, newError(
			EBadArgument,
			"the mock facility doesn't support uploading %s images to %s disks,"+
				" please upload in the disk format in your tests.",
			imageFormat,
			disk.Format(),
		)
	}

	progress := &mockImageUploadProgress{
		err:    nil,
		disk:   disk,
		client: m,
		reader: reader,
		size:   size,
		done:   make(chan struct{}),
	}

	// Lock the disk to simulate the upload being initialized.
	if err := progress.disk.Lock(); err != nil {
		return nil, newError(EDiskLocked, "disk locked after creation")
	}

	go progress.do()

	return progress, nil
}

func (m *mockClient) UploadToDisk(diskID string, size uint64, reader io.ReadSeekCloser, retries ...RetryStrategy) error {
	progress, err := m.StartUploadToDisk(diskID, size, reader, retries...)
	if err != nil {
		return err
	}
	<-progress.Done()
	return progress.Err()
}

func (m *mockClient) StartUploadToNewDisk(
	storageDomainID string,
	format ImageFormat,
	size uint64,
	params CreateDiskOptionalParameters,
	reader io.ReadSeekCloser,
	_ ...RetryStrategy,
) (UploadImageProgress, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, ok := m.storageDomains[storageDomainID]; !ok {
		return nil, newError(ENotFound, "storage domain with ID %s not found", storageDomainID)
	}

	imageFormat, qcowSize, err := extractQCOWParameters(size, reader)
	if err != nil {
		return nil, err
	}

	if imageFormat != format {
		return nil, newError(
			EBadArgument,
			"the mock facility doesn't support uploading %s images to %s disks,"+
				" please upload in the disk format in your tests.",
			imageFormat,
			format,
		)
	}

	disk, err := m.createDisk(storageDomainID, format, qcowSize, params)
	if err != nil {
		return nil, err
	}
	// Unlock the disk to simulate disk creation being complete.
	disk.Unlock()

	progress := &mockImageUploadProgress{
		err:    nil,
		disk:   disk,
		client: m,
		reader: reader,
		size:   size,
		done:   make(chan struct{}),
	}

	// Lock the disk to simulate the upload being initialized.
	if err := progress.disk.Lock(); err != nil {
		return nil, newError(EDiskLocked, "disk locked after creation")
	}

	go progress.do()

	return progress, nil
}

func (m *mockClient) UploadToNewDisk(
	storageDomainID string,
	format ImageFormat,
	size uint64,
	params CreateDiskOptionalParameters,
	reader io.ReadSeekCloser,
	retries ...RetryStrategy,
) (UploadImageResult, error) {
	progress, err := m.StartUploadToNewDisk(storageDomainID, format, size, params, reader, retries...)
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
	client        *mockClient
	reader        io.ReadSeekCloser
	size          uint64
	uploadedBytes uint64
	done          chan struct{}
}

func (m *mockImageUploadProgress) Disk() Disk {
	disk := m.disk
	if disk.id == "" {
		return nil
	}
	return disk
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
