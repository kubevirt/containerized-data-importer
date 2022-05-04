package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) StartVM(id string, retries ...RetryStrategy) (err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	err = retry(
		fmt.Sprintf("starting VM %s", id),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().VmsService().VmService(id).Start().Send()
			return err
		})
	return
}
