package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) ShutdownVM(id string, force bool, retries ...RetryStrategy) (err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	err = retry(
		fmt.Sprintf("shutting down VM %s", id),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().VmsService().VmService(id).Shutdown().Force(force).Send()
			return err
		})
	return
}
