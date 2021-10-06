package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) RemoveDisk(diskID string, retries ...RetryStrategy) error {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	return retry(
		fmt.Sprintf("removing disk %s", diskID),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().DisksService().DiskService(diskID).Remove().Send()
			return err
		},
	)
}
