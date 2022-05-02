package ovirtclient

func (m *mockClient) GetDiskAttachment(vmID string, diskAttachmentID string, _ ...RetryStrategy) (DiskAttachment, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	vm, ok := m.vmDiskAttachmentsByVM[vmID]
	if !ok {
		return nil, newError(ENotFound, "VM %s doesn't exist", vmID)
	}

	diskAttachment, ok := vm[diskAttachmentID]
	if !ok {
		return nil, newError(ENotFound, "disk attachment %s not found on VM %s", diskAttachmentID, vmID)
	}

	return diskAttachment, nil
}
