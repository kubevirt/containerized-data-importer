// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) GetVNICProfile(id string, retries ...RetryStrategy) (result VNICProfile, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("getting VNIC profile %s", id),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.SystemService().VnicProfilesService().ProfileService(id).Get().Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.Profile()
			if !ok {
				return newError(
					ENotFound,
					"no VNIC profile returned when getting VNIC profile ID %s",
					id,
				)
			}
			result, err = convertSDKVNICProfile(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert VNIC profile %s",
					id,
				)
			}
			return nil
		})
	return
}
