package ovirtclient

func (m *mockClient) RemoveTag(id string, _ ...RetryStrategy) (err error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, ok := m.tags[id]; !ok {
		return newError(ENotFound, "Tag with ID %s not found", id)
	}

	// remove the tag from all the VMs.
	for _, vm := range m.vms {
		for i, tagID := range vm.tagIDs {
			if tagID == id {
				// gocritic will complain on the following line due to appendAssign, but that's legit here
				m.vms[vm.id].tagIDs = append(vm.tagIDs[:i], vm.tagIDs[i+1:]...) //nolint:gocritic
				break
			}
		}
	}

	delete(m.tags, id)

	return nil
}
