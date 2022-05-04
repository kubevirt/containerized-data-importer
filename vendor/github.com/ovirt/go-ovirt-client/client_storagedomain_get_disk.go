package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) GetDiskFromStorageDomain(id string, diskID string, retries ...RetryStrategy) (result Disk, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("getting disk %s from storage domain %s", diskID, id),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.SystemService().StorageDomainsService().
				StorageDomainService(id).DisksService().DiskService(diskID).Get().Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.Disk()
			if !ok {
				return newError(
					ENotFound,
					"disk %s not found on storage domain ID %s",
					diskID,
					id,
				)
			}
			result, err = convertSDKDisk(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert disk %s",
					diskID,
				)
			}
			return nil
		})
	return result, err
}
