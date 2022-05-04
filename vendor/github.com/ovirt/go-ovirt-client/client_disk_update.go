package ovirtclient

import (
	"fmt"
	"sync"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

func (o *oVirtClient) UpdateDisk(id string, params UpdateDiskParameters, retries ...RetryStrategy) (
	result Disk,
	err error,
) {
	progress, err := o.StartUpdateDisk(id, params, retries...)
	if err != nil {
		return progress.Disk(), err
	}
	return progress.Wait(retries...)
}

func (o *oVirtClient) StartUpdateDisk(id string, params UpdateDiskParameters, retries ...RetryStrategy) (
	DiskUpdate,
	error,
) {
	retries = defaultRetries(retries, defaultWriteTimeouts())

	sdkDisk := ovirtsdk.NewDiskBuilder().Id(id)
	if alias := params.Alias(); alias != nil {
		sdkDisk.Alias(*alias)
	}
	if provisionedSize := params.ProvisionedSize(); provisionedSize != nil {
		sdkDisk.ProvisionedSize(int64(*provisionedSize))
	}
	correlationID := fmt.Sprintf("disk_update_%s", generateRandomID(5, o.nonSecureRandom))

	var disk Disk

	err := retry(
		fmt.Sprintf("updating disk %s", id),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.
				SystemService().
				DisksService().
				DiskService(id).
				Update().
				Disk(sdkDisk.MustBuild()).
				Query("correlation_id", correlationID).
				Send()
			if err != nil {
				return err
			}
			sdkDisk, ok := response.Disk()
			if !ok {
				return newError(
					EFieldMissing,
					"missing disk object from disk update response",
				)
			}
			disk, err = convertSDKDisk(sdkDisk, o)
			if err != nil {
				return wrap(err, EUnidentified, "failed to convert SDK disk object")
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return &diskWait{
		client:        o,
		disk:          disk,
		correlationID: correlationID,
		lock:          &sync.Mutex{},
	}, nil
}
