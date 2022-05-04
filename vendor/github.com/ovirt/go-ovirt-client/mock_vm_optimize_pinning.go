package ovirtclient

func (m *mockClient) AutoOptimizeVMCPUPinningSettings(_ string, _ bool, _ ...RetryStrategy) error {
	// This function cannot be simulated as the VM object does not contain any observable return values apart from the
	// NUMA nodes being moved around. If you know of a way please add a mock and add a test for it.
	return nil
}
