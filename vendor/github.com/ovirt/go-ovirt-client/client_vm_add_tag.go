package ovirtclient

import (
	"fmt"

	ovirtsdk "github.com/ovirt/go-ovirt"
)

func (o *oVirtClient) AddTagToVM(id string, tagID string, retries ...RetryStrategy) (err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	err = retry(
		fmt.Sprintf("removing VM %s", id),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().VmsService().VmService(id).TagsService().Add().
				Tag(ovirtsdk.NewTagBuilder().Id(tagID).MustBuild()).Send()

			if err != nil {
				return err
			}
			return nil
		})
	return
}

func (o *oVirtClient) AddTagToVMByName(id string, tagName string, retries ...RetryStrategy) (err error) {
	retries = defaultRetries(retries, defaultWriteTimeouts())
	err = retry(
		fmt.Sprintf("removing VM %s", id),
		o.logger,
		retries,
		func() error {
			_, err := o.conn.SystemService().VmsService().VmService(id).TagsService().Add().
				Tag(ovirtsdk.NewTagBuilder().Name(tagName).MustBuild()).Send()

			return err
		})
	return
}
