package ovirtclient

import (
	"fmt"
	"sync"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

func (o *oVirtClient) CopyTemplateDiskToStorageDomain(
	diskID string,
	storageDomainID string,
	retries ...RetryStrategy) (result Disk, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	progress, err := o.StartCopyTemplateDiskToStorageDomain(diskID, storageDomainID, retries...)
	if err != nil {
		return nil, err
	}

	return progress.Wait()
}

func (o *oVirtClient) StartCopyTemplateDiskToStorageDomain(
	diskID string,
	storageDomainID string,
	retries ...RetryStrategy) (DiskUpdate, error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	correlationID := fmt.Sprintf("template_disk_copy_%s", generateRandomID(5, o.nonSecureRandom))
	sdkStorageDomain := ovirtsdk.NewStorageDomainBuilder().Id(storageDomainID)
	sdkDisk := ovirtsdk.NewDiskBuilder().Id(diskID)
	storageDomain, _ := o.GetStorageDomain(storageDomainID)
	disk, _ := o.GetDisk(diskID)

	err := retry(
		fmt.Sprintf("copying disk %s to storage domain %s", diskID, storageDomainID),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.
				SystemService().
				DisksService().
				DiskService(diskID).
				Copy().
				StorageDomain(sdkStorageDomain.MustBuild()).
				Disk(sdkDisk.MustBuild()).
				Query("correlation_id", correlationID).
				Send()

			if err != nil {
				return err
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return &storageDomainDiskWait{
		client:        o,
		disk:          disk,
		storageDomain: storageDomain,
		correlationID: correlationID,
		lock:          &sync.Mutex{},
	}, nil
}
