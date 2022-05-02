package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) RemoveDiskFromStorageDomain(id string, diskID string, retries ...RetryStrategy) (err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("removing disk %s from storage domain %s", diskID, id),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().StorageDomainsService().
				StorageDomainService(id).DisksService().DiskService(diskID).Remove().Send()
			if err != nil {
				o.logger.Infof("error removing disk..")
				return err
			}

			return nil
		})
	return
}
