package ovirtclient

func (m *mockClient) SearchVMs(params VMSearchParameters, _ ...RetryStrategy) ([]VM, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	// We disable the "prealloc" linter here because it recommends preallocating result, which will lead
	// to inefficient memory usage.
	var result []VM //nolint:prealloc
	for _, vm := range m.vms {
		if name := params.Name(); name != nil && vm.name != *name {
			continue
		}
		if statuses := params.Statuses(); statuses != nil {
			foundStatus := false
			for _, status := range *statuses {
				if vm.Status() == status {
					foundStatus = true
					break
				}
			}
			if !foundStatus {
				continue
			}
		}
		if statuses := params.NotStatuses(); statuses != nil {
			foundStatus := false
			for _, status := range *statuses {
				if vm.Status() == status {
					foundStatus = true
					break
				}
			}
			if foundStatus {
				continue
			}
		}
		result = append(result, vm)
	}
	return result, nil
}
