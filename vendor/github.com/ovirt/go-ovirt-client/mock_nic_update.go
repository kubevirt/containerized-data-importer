package ovirtclient

func (m *mockClient) UpdateNIC(vmid string, nicID string, params UpdateNICParameters, retries ...RetryStrategy) (
	NIC,
	error,
) {
	m.lock.Lock()
	defer m.lock.Unlock()
	nic, ok := m.nics[nicID]
	if !ok {
		return nil, newError(ENotFound, "NIC not found")
	}
	if nic.vmid != vmid {
		return nil, newError(ENotFound, "NIC not found")
	}
	if name := params.Name(); name != nil {
		nic = nic.withName(*name)
	}
	if vnicProfileID := params.VNICProfileID(); vnicProfileID != nil {
		if _, ok := m.vnicProfiles[*vnicProfileID]; !ok {
			return nil, newError(ENotFound, "VNIC profile %s not found", *vnicProfileID)
		}
		nic = nic.withVNICProfileID(*vnicProfileID)
	}
	m.nics[nicID] = nic

	return nic, nil
}
