package ovirtclient

import (
	"fmt"
)

func (o *oVirtClient) RemoveTag(tagID string, retries ...RetryStrategy) (err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	err = retry(
		fmt.Sprintf("removing tag %s", tagID),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().TagsService().TagService(tagID).Remove().Send()
			return err
		})
	return
}
