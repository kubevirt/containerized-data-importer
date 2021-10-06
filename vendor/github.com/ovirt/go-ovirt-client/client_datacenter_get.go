// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) GetDatacenter(id string, retries ...RetryStrategy) (result Datacenter, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("getting datacenter %s", id),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.SystemService().DataCentersService().DataCenterService(id).Get().Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.DataCenter()
			if !ok {
				return newError(
					ENotFound,
					"no datacenter returned when getting datacenter ID %s",
					id,
				)
			}
			result, err = convertSDKDatacenter(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert datacenter %s",
					id,
				)
			}
			return nil
		})
	return
}
