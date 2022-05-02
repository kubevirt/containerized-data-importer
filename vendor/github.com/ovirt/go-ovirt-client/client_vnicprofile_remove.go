package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) RemoveVNICProfile(id string, retries ...RetryStrategy) (err error) {
	retries = defaultRetries(retries, defaultReadTimeouts())
	err = retry(
		fmt.Sprintf("removing VNIC profile %s", id),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().VnicProfilesService().ProfileService(id).Remove().Send()
			if err != nil {
				return err
			}
			return nil
		})
	return
}
