package ovirtclient

func (m *mockClient) ListDiskAttachments(vmID string, _ ...RetryStrategy) ([]DiskAttachment, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	diskAttachments, ok := m.vmDiskAttachmentsByVM[vmID]
	if !ok {
		return nil, newError(ENotFound, "VM %s doesn't exist", vmID)
	}

	result := make([]DiskAttachment, len(diskAttachments))
	i := 0
	for _, attachment := range diskAttachments {
		result[i] = attachment
		i++
	}

	return result, nil
}
