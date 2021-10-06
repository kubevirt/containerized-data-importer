// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) GetDisk(id string, retries ...RetryStrategy) (result Disk, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("getting disk %s", id),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.SystemService().DisksService().DiskService(id).Get().Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.Disk()
			if !ok {
				return newError(
					ENotFound,
					"no disk returned when getting disk ID %s",
					id,
				)
			}
			result, err = convertSDKDisk(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert disk %s",
					id,
				)
			}
			return nil
		})
	return
}
