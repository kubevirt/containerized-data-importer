package ovirtclient

import "fmt"

func (o *oVirtClient) AutoOptimizeVMCPUPinningSettings(id string, optimize bool, retries ...RetryStrategy) error {
	return retry(
		fmt.Sprintf("optimizing CPU pinning settings for VM %s", id),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().
				VmsService().
				VmService(id).
				AutoPinCpuAndNumaNodes().
				OptimizeCpuSettings(optimize).
				Send()
			return err
		})
}
