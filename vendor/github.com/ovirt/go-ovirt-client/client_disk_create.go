package ovirtclient

import (
	"fmt"
	"sync"

	ovirtsdk4 "github.com/ovirt/go-ovirt"
)

func (o *oVirtClient) StartCreateDisk(
	storageDomainID string,
	format ImageFormat,
	size uint64,
	params CreateDiskOptionalParameters,
	retries ...RetryStrategy,
) (DiskCreation, error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())

	if err := validateDiskCreationParameters(format, size); err != nil {
		return nil, err
	}

	var result *diskWait
	processName := "creating disk"
	correlationID := ""
	if params != nil && params.Alias() != "" {
		processName = fmt.Sprintf("creating disk %s", params.Alias())
		correlationID = fmt.Sprintf("disk_create_%s", params.Alias())
	} else {
		correlationID = fmt.Sprintf("disk_create_%s", generateRandomID(5, o.nonSecureRandom))
	}
	err := retry(
		processName,
		o.logger,
		retries,
		func() error {
			addResponse, err := o.createDisk(storageDomainID, size, format, correlationID, params)
			if err != nil {
				return err
			}
			sdkDisk, ok := addResponse.Disk()
			if !ok {
				return newError(
					EFieldMissing,
					"missing disk object from disk add response",
				)
			}
			resultDisk, err := convertSDKDisk(sdkDisk, o)
			if err != nil {
				return wrap(err, EUnidentified, "failed to convert SDK disk object")
			}

			result = &diskWait{
				lock:          &sync.Mutex{},
				client:        o,
				disk:          resultDisk,
				correlationID: correlationID,
			}
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func validateDiskCreationParameters(format ImageFormat, size uint64) error {
	if err := format.Validate(); err != nil {
		return err
	}
	if size < 1048576 {
		// 1M is the minimum size for disks in oVirt. Smaller disks can be created, but they lead to bugs in oVirt
		// when creating disks from templates and changing the format.
		return newError(EBadArgument, "Disk size must be at least 1048576 bytes (1 MB)")
	}
	return nil
}

func (o *oVirtClient) createDisk(
	storageDomainID string,
	size uint64,
	format ImageFormat,
	correlationID string,
	params CreateDiskOptionalParameters,
) (*ovirtsdk4.DisksServiceAddResponse, error) {
	disk, err := o.buildDiskObjectForCreation(storageDomainID, size, format, params)
	if err != nil {
		return nil, wrap(
			err,
			EBug,
			"failed to construct disk object",
		)
	}
	return o.conn.
		SystemService().
		DisksService().
		Add().
		Disk(disk).
		Query("correlation_id", correlationID).
		Send()
}

func (o *oVirtClient) buildDiskObjectForCreation(
	storageDomainID string,
	size uint64,
	format ImageFormat,
	params CreateDiskOptionalParameters,
) (*ovirtsdk4.Disk, error) {
	storageDomain, err := ovirtsdk4.NewStorageDomainBuilder().Id(storageDomainID).Build()
	if err != nil {
		return nil, wrap(
			err,
			EBug,
			"failed to build storage domain object from storage domain ID: %s",
			storageDomainID,
		)
	}
	diskBuilder := ovirtsdk4.NewDiskBuilder().
		ProvisionedSize(int64(size)).
		InitialSize(int64(size)).
		StorageDomainsOfAny(storageDomain).
		Format(ovirtsdk4.DiskFormat(format))
	if params != nil {
		if sparse := params.Sparse(); sparse != nil {
			diskBuilder.Sparse(*sparse)
		}
		if alias := params.Alias(); alias != "" {
			diskBuilder.Alias(alias)
		}
	}
	return diskBuilder.Build()
}

func (o *oVirtClient) CreateDisk(
	storageDomainID string,
	format ImageFormat,
	size uint64,
	params CreateDiskOptionalParameters,
	retries ...RetryStrategy,
) (Disk, error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	waitRetries := defaultRetries(retries, defaultLongTimeouts())
	result, err := o.StartCreateDisk(storageDomainID, format, size, params, retries...)
	if err != nil {
		return nil, err
	}
	disk, err := result.Wait(waitRetries...)
	if err != nil {
		o.logger.Warningf("Created disk %s, but failed to wait for it to unlock. (%v)", result.Disk().ID(), err)
	}
	return disk, err
}
