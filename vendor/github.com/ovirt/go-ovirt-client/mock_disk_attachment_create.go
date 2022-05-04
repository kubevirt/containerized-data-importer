package ovirtclient

func (m *mockClient) CreateDiskAttachment(
	vmID string,
	diskID string,
	diskInterface DiskInterface,
	params CreateDiskAttachmentOptionalParams,
	_ ...RetryStrategy,
) (DiskAttachment, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if err := diskInterface.Validate(); err != nil {
		return nil, wrap(err, EBadArgument, "failed to create disk attachment")
	}

	vm, ok := m.vms[vmID]
	if !ok {
		return nil, newError(ENotFound, "VM with ID %s not found", vmID)
	}

	disk, ok := m.disks[diskID]
	if !ok {
		return nil, newError(ENotFound, "disk with ID %s not found", diskID)
	}

	attachment := &diskAttachment{
		client:        m,
		id:            m.GenerateUUID(),
		vmid:          vm.ID(),
		diskID:        disk.ID(),
		diskInterface: diskInterface,
	}
	attachment.active = true
	if params != nil {
		if bootable := params.Bootable(); bootable != nil {
			attachment.bootable = *bootable
		}
		if active := params.Active(); active != nil {
			attachment.active = *active
		}
	}
	for _, diskAttachment := range m.vmDiskAttachmentsByVM[vm.ID()] {
		if diskAttachment.DiskID() == diskID {
			return nil, newError(EConflict, "disk %s is already attached to VM %s", diskID, vmID)
		}
	}

	if diskAttachment, ok := m.vmDiskAttachmentsByDisk[disk.ID()]; ok {
		return nil, newError(
			EConflict,
			"cannot attach disk %s to VM %s, already attached to VM %s",
			diskID,
			vmID,
			diskAttachment.VMID(),
		)
	}

	m.vmDiskAttachmentsByDisk[disk.ID()] = attachment
	m.vmDiskAttachmentsByVM[vm.ID()][attachment.ID()] = attachment

	return attachment, nil
}
