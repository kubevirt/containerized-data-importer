package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) RemoveVM(id string, retries ...RetryStrategy) (err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	err = retry(
		fmt.Sprintf("removing VM %s", id),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().VmsService().VmService(id).Remove().Send()
			if err != nil {
				return err
			}
			return nil
		})
	return
}
