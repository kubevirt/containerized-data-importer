package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) StopVM(id string, force bool, retries ...RetryStrategy) (err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	err = retry(
		fmt.Sprintf("stopping VM %s", id),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().VmsService().VmService(id).Stop().Force(force).Send()
			return err
		})
	return
}
