package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) RemoveNIC(vmid string, id string, retries ...RetryStrategy) (err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	err = retry(
		fmt.Sprintf("removing NIC %s from VM %s", id, vmid),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().VmsService().VmService(vmid).NicsService().NicService(id).Remove().Send()
			if err != nil {
				return err
			}
			return nil
		})
	return
}
