package ovirtclient

import "time"

func (m *mockClient) CopyTemplateDiskToStorageDomain(
	diskID string,
	storageDomainID string,
	retries ...RetryStrategy) (result Disk, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	disk, ok := m.disks[diskID]

	if !ok {
		return nil, newError(ENotFound, "disk with ID %s not found", diskID)
	}
	if err := disk.Lock(); err != nil {
		return nil, err
	}
	update := &mockDiskCopy{
		client:          m,
		disk:            disk,
		storageDomainID: storageDomainID,
		done:            make(chan struct{}),
	}
	defer update.do()
	return disk, nil
}

type mockDiskCopy struct {
	client          *mockClient
	disk            *diskWithData
	storageDomainID string
	done            chan struct{}
}

func (c *mockDiskCopy) Disk() Disk {
	c.client.lock.Lock()
	defer c.client.lock.Unlock()

	return c.disk
}

func (c *mockDiskCopy) Wait(_ ...RetryStrategy) (Disk, error) {
	<-c.done

	return c.disk, nil
}

func (c *mockDiskCopy) do() {
	// Sleep to trigger potential race conditions / improper status handling.
	time.Sleep(time.Second)
	c.client.disks[c.disk.ID()] = c.disk
	c.client.disks[c.disk.ID()].storageDomainIDs = append(c.client.disks[c.disk.ID()].storageDomainIDs, c.storageDomainID)
	close(c.done)
}
