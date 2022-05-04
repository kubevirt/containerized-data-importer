package ovirtclient

func (m *mockClient) RemoveDiskAttachment(vmID string, diskAttachmentID string, _ ...RetryStrategy) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	vm, ok := m.vmDiskAttachmentsByVM[vmID]
	if !ok {
		return newError(ENotFound, "VM %s doesn't exist", vmID)
	}

	diskAttachment, ok := vm[diskAttachmentID]
	if !ok {
		return newError(ENotFound, "Disk attachment %s not found on VM %s", diskAttachmentID, vmID)
	}

	delete(m.vmDiskAttachmentsByDisk, diskAttachment.DiskID())
	delete(m.vmDiskAttachmentsByVM[vmID], diskAttachmentID)

	return nil
}
