// Code generated automatically using go:generate. DO NOT EDIT.

package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) GetVM(id string, retries ...RetryStrategy) (result VM, err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("getting vm %s", id),
		o.logger,
		retries,
		func() error {
			response, err := o.conn.SystemService().VmsService().VmService(id).Get().Send()
			if err != nil {
				return err
			}
			sdkObject, ok := response.Vm()
			if !ok {
				return newError(
					ENotFound,
					"no vm returned when getting vm ID %s",
					id,
				)
			}
			result, err = convertSDKVM(sdkObject, o)
			if err != nil {
				return wrap(
					err,
					EBug,
					"failed to convert vm %s",
					id,
				)
			}
			return nil
		})
	return
}
