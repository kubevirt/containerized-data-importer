package ovirtclient

func (m *mockClient) UpdateVM(id string, params UpdateVMParameters, _ ...RetryStrategy) (VM, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, ok := m.vms[id]; !ok {
		return nil, newError(ENotFound, "VM with ID %s not found", id)
	}

	vm := m.vms[id]
	if name := params.Name(); name != nil {
		for _, otherVM := range m.vms {
			if otherVM.name == *name && otherVM.ID() != vm.ID() {
				return nil, newError(EConflict, "A VM with the name \"%s\" already exists.", *name)
			}
		}
		vm = vm.withName(*name)
	}
	if comment := params.Comment(); comment != nil {
		vm = vm.withComment(*comment)
	}
	m.vms[id] = vm

	return vm, nil
}
