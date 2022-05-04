package ovirtclient

import (
	"time"
)

func (m *mockClient) UpdateDisk(id string, params UpdateDiskParameters, retries ...RetryStrategy) (Disk, error) {
	progress, err := m.StartUpdateDisk(id, params, retries...)
	if err != nil {
		return progress.Disk(), err
	}
	return progress.Wait(retries...)
}

func (m *mockClient) StartUpdateDisk(id string, params UpdateDiskParameters, _ ...RetryStrategy) (
	DiskUpdate,
	error,
) {
	m.lock.Lock()
	defer m.lock.Unlock()

	var err error
	disk, ok := m.disks[id]
	if !ok {
		return nil, newError(ENotFound, "disk with ID %s not found", id)
	}
	if err := disk.Lock(); err != nil {
		return nil, err
	}
	if alias := params.Alias(); alias != nil {
		disk = disk.WithAlias(alias)
	}
	if ps := params.ProvisionedSize(); ps != nil {
		disk, err = disk.withProvisionedSize(*ps)
		if err != nil {
			return nil, err
		}
	}
	update := &mockDiskUpdate{
		client: m,
		disk:   disk,
		done:   make(chan struct{}),
	}
	defer update.do()
	return update, nil
}

type mockDiskUpdate struct {
	client *mockClient
	disk   *diskWithData
	done   chan struct{}
}

func (c *mockDiskUpdate) Disk() Disk {
	c.client.lock.Lock()
	defer c.client.lock.Unlock()

	return c.disk
}

func (c *mockDiskUpdate) Wait(_ ...RetryStrategy) (Disk, error) {
	<-c.done

	return c.disk, nil
}

func (c *mockDiskUpdate) do() {
	// Sleep to trigger potential race conditions / improper status handling.
	time.Sleep(time.Second)

	c.client.disks[c.disk.ID()] = c.disk
	c.disk.Unlock()

	close(c.done)
}
