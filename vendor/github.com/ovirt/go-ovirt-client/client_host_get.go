// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) GetHost(id string, retries ...RetryStrategy) (result Host, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("getting host %s", id),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.SystemService().HostsService().HostService(id).Get().Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.Host()
			if !ok {
				return newError(
					ENotFound,
					"no host returned when getting host ID %s",
					id,
				)
			}
			result, err = convertSDKHost(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert host %s",
					id,
				)
			}
			return nil
		})
	return
}
