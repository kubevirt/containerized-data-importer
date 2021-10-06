// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) GetNetwork(id string, retries ...RetryStrategy) (result Network, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("getting network %s", id),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.SystemService().NetworksService().NetworkService(id).Get().Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.Network()
			if !ok {
				return newError(
					ENotFound,
					"no network returned when getting network ID %s",
					id,
				)
			}
			result, err = convertSDKNetwork(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert network %s",
					id,
				)
			}
			return nil
		})
	return
}
