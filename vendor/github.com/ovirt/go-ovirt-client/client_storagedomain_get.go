// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) GetStorageDomain(id string, retries ...RetryStrategy) (result StorageDomain, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("getting storage domain %s", id),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.SystemService().StorageDomainsService().StorageDomainService(id).Get().Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.StorageDomain()
			if !ok {
				return newError(
					ENotFound,
					"no storage domain returned when getting storage domain ID %s",
					id,
				)
			}
			result, err = convertSDKStorageDomain(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert storage domain %s",
					id,
				)
			}
			return nil
		})
	return
}
